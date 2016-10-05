// Copyright 2016 zxfonline@sina.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package nbtcp

import (
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/zxfonline/chanutil"
	"github.com/zxfonline/golog"
	"github.com/zxfonline/shutdown"
	"github.com/zxfonline/taskexcutor"
	"github.com/zxfonline/timefix"
	"golang.org/x/net/context"
)

var (
	connLogger = golog.New("Connect")
)

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
	//收到消息是否阻塞
	runstate chan bool
	runlock  sync.Mutex
	//消息收取缓冲管道
	readChan chan IoBuffer
	//消息发送器
	sender  *Sender
	service *taskexcutor.TaskService
}

//构建tcp连接
func CreateConnect(wg *sync.WaitGroup, conn net.Conn, ChanReadSize, ChanSendSize, DeadlineSecond, ReadBufMaxSize uint) *Connect {
	addr := conn.RemoteAddr().String()
	pos := strings.LastIndex(addr, ":")
	if pos > 0 {
		addr = addr[0:pos]
	}
	c := new(Connect)
	c.wg = wg
	c.conn = conn
	c.ip = net.ParseIP(addr).String()
	c.ChanReadSize = ChanReadSize
	c.ChanSendSize = ChanSendSize
	c.DeadlineSecond = DeadlineSecond
	c.bufMaxSize = ReadBufMaxSize
	connLogger.Debugf("CREATE %+v", c)
	return c
}

//socket消息发送统一方法，由消息发送器调用
func (c *Connect) transfer(out interface{}) {
	defer c.recoverClose()
	select {
	case <-c.stopD: //连接关闭
		connLogger.Debugf("write data to closed connect,data=%+v,conn=%+v", out, c)
	default:
		switch v := out.(type) {
		case IoBuffer:
			c.rw.WriteMsg(c.filter.MessageSend(c, v))
		default: //非IoBuffer 表示关闭连接
			c.Close()
		}
	}
}

//socket消息处理统一方法，由线程池调用,其他地方请勿调用
func (c *Connect) Processing(params ...interface{}) {
	defer c.recoverClose()
	defer func() {
		c.runlock.Lock()
		<-c.runstate
		c.runlock.Unlock()
	}()
	for q := false; !q; {
		select {
		case <-c.stopD: //连接关闭
			q = true
		default:
			select {
			case data := <-c.readChan:
				data.SetPrct(timefix.MillisTime())
				c.transH.Transmit(c, c.filter.MessageReceived(c, data))
			default:
				q = true
			}
		}
	}
}

func (c *Connect) openCheck() (ok bool) {
	defer c.recoverClose()
	defer func() {
		if !ok {
			connLogger.Warnf("INIT FALSE %+v", c)
			c.Close()
		}
	}()
	ok = c.filter.SessionOpening(c)
	return
}

//tcp连接初始化
func (c *Connect) Open(parent context.Context, msgExcutor taskexcutor.Excutor, cid int64, msgProcessor MsgHandler, ioc func(io.ReadWriter, IoSession) MsgReadWriter, iofilterRegister func(IoSession) IoFilterChain) (ok bool) {
	c.initOnce.Do(func() {
		connLogger.Debugf("INITING %+v", c)
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
		c.sender = NewSender(c)
		go c.sender.Start()
		c.transH = msgProcessor
		c.readChan = make(chan IoBuffer, c.ChanReadSize)
		c.service = taskexcutor.NewTaskService(c.Processing)
		c.runstate = make(chan bool, 1)
		go c.monitor(ctx)
		go c.receiving(ctx, msgExcutor)

		defer c.recoverClose()
		c.filter.SessionOpened(c)
		connLogger.Debugf("INITED %+v", c)
		ok = true
	})
	return
}

func (c *Connect) monitor(ctx context.Context) {
	defer c.Close()
	defer func() {
		if e := recover(); e != nil {
			connLogger.Debugf("recover error:%v,conn=%+v", e, c)
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

func (c *Connect) receiving(ctx context.Context, msgExcutor taskexcutor.Excutor) {
	defer c.Close()
	defer func() {
		if e := recover(); e != nil {
			connLogger.Debugf("recover error:%v,conn=%+v", e, c)
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
			c.readChan <- data
			c.runlock.Lock()
			select {
			case c.runstate <- true:
				c.runlock.Unlock()
				err := msgExcutor.Excute(c.service)
				if err != nil {
					panic(err)
				}
			default:
				c.runlock.Unlock()
			}
		}
	}
}

func (c *Connect) recoverClose() {
	if e := recover(); e != nil {
		if e == io.EOF {
			connLogger.Debugf("recover error:%v,conn=%+v", e, c)
		} else {
			connLogger.Warnf("recover error:%v,conn=%+v", e, c)
		}
		c.Close()
	}
}

func (c *Connect) recoverLog() {
	if e := recover(); e != nil {
		connLogger.Warnf("recover error:%v,conn=%+v", e, c)
	}
}

//shutdown.StopNotifier.Close()
func (c *Connect) Close() {
	c.closeOnce.Do(func() {
		connLogger.Debugf("STOPING %+v", c)
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
			connLogger.Debugf("STOPED %+v", c)
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
		connLogger.Debugf("write data to connect sender error=%v,data=%+v,conn=%+v", err, out, c)
	}
}
