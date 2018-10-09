// Copyright 2016 zxfonline@sina.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"io"
	"net"
	"sync"
	"time"

	"context"

	"github.com/zxfonline/chanutil"
	"github.com/zxfonline/golog"
	. "github.com/zxfonline/net/packet"
	"github.com/zxfonline/shutdown"
	"github.com/zxfonline/taskexcutor"
	"github.com/zxfonline/timefix"
)

var logger *golog.Logger = golog.New("Connect")

//构建tcp连接
func CreateConnect(wg *sync.WaitGroup, conn net.Conn, ChanReadSize, ChanSendSize, DeadlineSecond, ReadBufMaxSize uint) *Connect {
	addr := conn.RemoteAddr().String()
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	c := new(Connect)
	c.wg = wg
	c.conn = conn
	c.ip = net.ParseIP(host).String()
	c.ChanReadSize = ChanReadSize
	c.ChanSendSize = ChanSendSize
	c.DeadlineSecond = DeadlineSecond
	c.bufMaxSize = ReadBufMaxSize
	logger.Debugf("CREATE %+v", c)
	return c
}

//tcp连接消息处理类，收发的消息通过管道进行排序处理，适用于客户端到服务器之间的连接进行处理
type Connect struct {
	//连接唯一id
	cid int64
	//连接通道
	conn net.Conn
	//连接地址
	ip string
	//管道读取数据缓冲大小
	ChanReadSize uint
	//管道发送数据缓冲大小
	ChanSendSize uint
	//连接空闲超时时间 秒 0表示不做空闲超时处理
	DeadlineSecond uint
	//消息包最大字节,0表示无上限
	bufMaxSize uint
	//消息处理器
	transH MsgHandler
	//消息过滤器
	filter IoFilterChain
	//消息读写器
	rw MsgReadWriter
	//关闭连接执行体
	closeOnce sync.Once
	//初始化连接执行体
	initOnce sync.Once
	wg       *sync.WaitGroup
	stopD    chanutil.DoneChan
	quitF    context.CancelFunc

	//收到消息是否阻塞，判断是否正在执行消息事件
	runstate chan bool
	//消息队列执行是否等待
	waitChan bool
	//消息收取缓冲管道
	readChan       chan IoBuffer
	service        *taskexcutor.TaskService
	serviceExcutor taskexcutor.Excutor

	//消息发送器
	sender *sender
	//连接绑定的的数据源
	source interface{}
}

//设置连接数据源
func (c *Connect) SetSource(source interface{}) {
	c.source = source
}

//获取连接数据源
func (c *Connect) GetSource() interface{} {
	return c.source
}

//socket消息发送统一方法，由消息发送器调用
func (c *Connect) transfer(out interface{}) {
	defer c.recoverClose()
	select {
	case <-c.stopD: //连接关闭
		logger.Debugf("write data to closed connect,data=%+v,conn=%+v", out, c)
	default:
		switch v := out.(type) {
		case IoBuffer:
			if c.filter.MessageSend(c, v) {
				c.rw.WriteMsg(v)
			}
		default: //非IoBuffer 表示关闭连接
			c.Close()
		}
	}
}

//消息处理事件中，wait=true 消息需要异步回调执行完成后再继续连接中后续的消息,wait=false 回调完成后需要关闭等待,否则该连接不再继续处理消息
func (c *Connect) QueueProcessWait(wait bool) {
	if wait && (!c.waitChan) {
		c.waitChan = true
	} else if (!wait) && c.waitChan {
		c.waitChan = false
		if len(c.readChan) > 0 {
			if c.serviceExcutor != nil {
				err := c.serviceExcutor.Excute(c.service)
				if err != nil {
					select {
					case <-c.runstate:
					default:
					}
				}
			}
		} else {
			select {
			case <-c.runstate:
			default:
			}
		}
	}
}

//socket消息处理统一方法，由线程池调用,其他地方请勿调用
func (c *Connect) processingQueue(params ...interface{}) {
	defer c.recoverClose()
	defer func() {
		if !c.waitChan {
			select {
			case <-c.runstate:
			default:
			}
		}
	}()
	for q := false; !q; {
		select {
		case <-c.stopD: //连接关闭
			q = true
		default:
			select {
			case data := <-c.readChan:
				func() {
					defer func() {
						if e := recover(); e != nil {
							data.TraceErrorf("process err:%v", e)
							data.TracePrintf("end process")
							data.TraceFinish()
							panic(e)
						} else {
							data.TracePrintf("end process")
							data.TraceFinish()
						}
					}()
					data.TracePrintf("start process")
					data.SetPrct(timefix.NanosTime())
					if c.filter.MessageReceived(c, data) {
						c.transH.Transmit(c, data)
					}
					if c.waitChan { //需要异步回调后处理后续消息
						q = true
					}
				}()
			default:
				q = true
			}
		}
	}
}

//socket消息处理统一方法，由线程池调用,其他地方请勿调用
func (c *Connect) processingMutil(params ...interface{}) {
	select {
	case <-c.stopD: //连接关闭
	default:
		data := params[0].(IoBuffer)
		defer c.recoverClose()
		defer func() {
			if e := recover(); e != nil {
				data.TraceErrorf("process err:%v", e)
				data.TracePrintf("end process")
				data.TraceFinish()
				panic(e)
			} else {
				data.TracePrintf("end process")
				data.TraceFinish()
			}
		}()
		data.TracePrintf("start process")
		data.SetPrct(timefix.NanosTime())
		if c.filter.MessageReceived(c, data) {
			c.transH.Transmit(c, data)
		}
	}
}

func (c *Connect) openCheck() (ok bool) {
	defer c.recoverClose()
	defer func() {
		if !ok {
			logger.Warnf("INIT FALSE %+v", c)
			c.Close()
		}
	}()
	ok = c.filter.SessionOpening(c)
	return
}

//tcp连接初始化
func (c *Connect) Open(parent context.Context, msgExcutor taskexcutor.Excutor, cid int64, msgProcessor MsgHandler, ioc func(io.ReadWriter, IoSession) MsgReadWriter, iofilterRegister func(IoSession) IoFilterChain, mutilMsg bool) (ok bool) {
	c.initOnce.Do(func() {
		logger.Debugf("INITING %+v", c)
		c.wg.Add(1)
		c.filter = iofilterRegister(c)
		next := c.openCheck()
		if !next {
			ok = false
			return
		}
		if parent == nil {
			parent = context.Background()
		}
		var ctx context.Context
		ctx, c.quitF = context.WithCancel(parent)

		c.stopD = chanutil.NewDoneChan()
		c.cid = cid
		c.rw = ioc(c.conn, c)
		c.serviceExcutor = msgExcutor
		c.sender = newsender(c)
		go c.sender.Start()
		c.transH = msgProcessor

		if mutilMsg {
			go c.receivingMutil(ctx, msgExcutor)
		} else {
			c.readChan = make(chan IoBuffer, c.ChanReadSize)
			c.runstate = make(chan bool, 1)
			c.service = taskexcutor.NewTaskService(c.processingQueue)
			go c.receivingQueue(ctx, msgExcutor)
		}
		go c.monitor(ctx)

		defer c.recoverClose()
		c.filter.SessionOpened(c)
		logger.Debugf("INITED %+v", c)
		ok = true
	})
	return
}

//初始化加密
func (c *Connect) InitEncrypt(token int64) {
	c.filter.InitEncrypt(token, func(encryptdata IoBuffer) {
		if encryptdata != nil {
			c.rw.WriteMsg(encryptdata)
		}
	})
}

func (c *Connect) monitor(ctx context.Context) {
	defer c.Close()
	defer func() {
		if e := recover(); e != nil {
			logger.Debugf("recover error:%v,conn=%+v", e, c)
		}
	}()
	for q := false; !q; {
		select {
		case <-ctx.Done():
			q = true
		case <-c.stopD:
			q = true
		}
	}
}

func (c *Connect) receivingQueue(ctx context.Context, msgExcutor taskexcutor.Excutor) {
	defer c.Close()
	defer func() {
		if e := recover(); e != nil {
			logger.Debugf("recover error:%v,conn=%+v", e, c)
		}
	}()
	for q := false; !q; {
		if c.DeadlineSecond > 0 {
			c.conn.SetReadDeadline(time.Now().Add(time.Duration(c.DeadlineSecond) * time.Second))
		}
		data := c.rw.ReadMsg()
		select {
		case <-ctx.Done(): //连接关闭
			q = true
		default:
			data.SetConnectID(c.cid)
			data.TracePrintf("wait process msg from %v", c.RemoteAddr())
			c.readChan <- data
			select {
			case c.runstate <- true:
				err := msgExcutor.Excute(c.service)
				if err != nil {
					panic(err)
				}
			default:
			}
		}
	}
}

func (c *Connect) receivingMutil(ctx context.Context, msgExcutor taskexcutor.Excutor) {
	defer c.Close()
	defer func() {
		if e := recover(); e != nil {
			logger.Debugf("recover error:%v,conn=%+v", e, c)
		}
	}()
	for q := false; !q; {
		if c.DeadlineSecond > 0 {
			c.conn.SetReadDeadline(time.Now().Add(time.Duration(c.DeadlineSecond) * time.Second))
		}
		data := c.rw.ReadMsg()
		select {
		case <-ctx.Done(): //连接关闭
			q = true
		default:
			data.SetConnectID(c.cid)
			data.TracePrintf("wait process msg from %v", c.RemoteAddr())
			err := msgExcutor.Excute(taskexcutor.NewTaskService(c.processingMutil, data))
			if err != nil {
				panic(err)
			}
		}
	}
}

func (c *Connect) recoverClose() {
	if e := recover(); e != nil {
		if e == io.EOF {
			logger.Debugf("recover error:%v,conn=%+v", e, c)
		} else {
			logger.Warnf("recover error:%v,conn=%+v", e, c)
		}
		c.Close()
	}
}

func (c *Connect) recoverLog() {
	if e := recover(); e != nil {
		logger.Warnf("recover error:%v,conn=%+v", e, c)
	}
}

//shutdown.StopNotifier.Close()
func (c *Connect) Close() {
	c.closeOnce.Do(func() {
		logger.Debugf("STOPING %+v", c)
		if c.quitF != nil {
			c.quitF()
		}
		if c.stopD != nil {
			c.stopD.SetDone()
		}
		if c.sender != nil {
			c.sender.Close()
		}
		c.conn.Close()
		if c.rw != nil {
			if sn, ok := c.rw.(shutdown.StopNotifier); ok {
				sn.Close()
			}
		}
		defer func() {
			logger.Debugf("STOPED %+v", c)
			c.serviceExcutor = nil
			c.wg.Done()
		}()
		defer c.recoverLog()
		//抛出消息
		c.filter.SessionClosed(c)
	})
}

//是否关闭
func (c *Connect) Closed() bool {
	return c.stopD.R().Done()
}

//消息包最大字节,0表示无上限
func (c *Connect) ReadBufMaxSize() uint {
	return c.bufMaxSize
}

//session.IoSession.GetCid()
func (c *Connect) GetCid() int64 {
	return c.cid
}

//session.IoSession.LocalAddr()
func (c *Connect) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

//session.IoSession.RemoteAddr()
func (c *Connect) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

//session.IoSession.RemoteIp()
func (c *Connect) RemoteIP() string {
	return c.ip
}

//session.IoSession.Write() 向连接写入消息，通过执行器对消息进行排队发送(非阻塞方式)
func (c *Connect) Write(out interface{}) {
	err := c.sender.Write(out)
	if err != nil {
		logger.Debugf("write data to connect sender error=%v,data=%+v,conn=%+v", err, out, c)
	}
}
