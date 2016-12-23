// Copyright 2016 zxfonline@sina.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package nbtcp

import (
	"sync"
	"time"

	"io"
	"net"

	"context"

	"github.com/zxfonline/chanutil"
	"github.com/zxfonline/golog"
	. "github.com/zxfonline/net/conn"
	. "github.com/zxfonline/net/packet"
	"github.com/zxfonline/taskexcutor"
	. "github.com/zxfonline/trace"
	"golang.org/x/net/trace"
)

var (
	MAXN_RETRY_TIMES               = 60
	clientFLogger    *golog.Logger = golog.New("ConnectClientFactory")
	serverLogger     *golog.Logger = golog.New("TCP_SERVICE")
)

/**
服务器基础属性参数
<pre>
{
	//服务器消息执行并发线程数
	"MultipleSize":1,
	//服务器消息线程消息管道缓冲上限
	"MultipleChanSize":10000,
	//连接管道读取数据缓冲大小
	"ChanReadSize":16,
	//连接管道发送数据缓冲大小
	"ChanSendSize":10000,
	//消息包最大字节数,0表示无上限
	"ReadBufMaxSize":20480,
	//连接空闲超时时间 秒 0 表示不做超时断开
	"DeadlineSecond":180
}
</pre>
*/
type ServerConfig struct {
	//服务器消息执行并发线程数
	MultipleSize uint `json:"MultipleSize"`
	//服务器消息线程消息管道缓冲上限
	MultipleChanSize uint `json:"MultipleChanSize"`
	//连接管道读取数据缓冲大小
	ChanReadSize uint `json:"ChanReadSize"`
	//连接管道发送数据缓冲大小
	ChanSendSize uint `json:"ChanSendSize"`
	//消息包最大字节数,0表示无上限
	ReadBufMaxSize uint `json:"ReadBufMaxSize"`
	//连接空闲超时时间 秒 0 表示不做超时断开
	DeadlineSecond uint `json:"DeadlineSecond"`
	//连接消息是否并发执行，true表示连接收到消息就丢到线程池，否则连接在执行消息任务期间收到的消息放入连接消息缓存中等待未完成的消息完毕继续执行(连接消息同时只能被一条线程执行)
	ConnectMutilMsg bool `json:"ConnectMutilMsg"`
}

func NewClientFactory(wg *sync.WaitGroup, config *ServerConfig, msgProcessor MsgHandler, ioc func(io.ReadWriter, IoSession) MsgReadWriter, iofilterRegister func(IoSession) IoFilterChain) *ConnectClientFactory {
	return &ConnectClientFactory{
		stopD:            chanutil.NewDoneChan(),
		config:           config,
		msgProcessor:     msgProcessor,
		ioc:              ioc,
		iofilterRegister: iofilterRegister,
		wg:               wg,
		connectMap:       make(map[string]IoSession),
		sessionChan:      make(chan IoSession),
		crtChan:          make(chan string),
	}
}

//创建新连接
func OpenConnect(ctx context.Context, address string, timeout time.Duration, wg *sync.WaitGroup, connId int64, msgProcessor MsgHandler, ioc func(io.ReadWriter, IoSession) MsgReadWriter, iofilterRegister func(IoSession) IoFilterChain, msgExcutor taskexcutor.Excutor, ChanReadSize, ChanSendSize, DeadlineSecond, ReadBufMaxSize uint, mutilMsg bool) IoSession {
	if address == "" {
		clientFLogger.Warnf("OpenInstance error,connect fail,nil address")
		return nil
	}
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		clientFLogger.Warnf("OpenInstance error,connect fail,error:%s", err)
		return nil
	}
	c := CreateConnect(wg, conn, ChanReadSize, ChanSendSize, DeadlineSecond, ReadBufMaxSize)
	ok := c.Open(ctx, msgExcutor, connId, msgProcessor, ioc, iofilterRegister, mutilMsg)
	if ok {
		clientFLogger.Infof("OpenInstance ok,connect opened,connect=%+v", c)
		return c
	}
	return nil
}

//初始化tcp服务器
func NewServer(wg *sync.WaitGroup, config *ServerConfig, msgProcessor MsgHandler, ioc func(io.ReadWriter, IoSession) MsgReadWriter, iofilterRegister func(IoSession) IoFilterChain, address string) *Server {
	s := &Server{
		stopD:            chanutil.NewDoneChan(),
		config:           config,
		msgProcessor:     msgProcessor,
		ioc:              ioc,
		iofilterRegister: iofilterRegister,
		wg:               wg,
		address:          address,
	}
	if EnableTracing {
		s.events = trace.NewEventLog("tcp.Server", address)
	}
	return s
}

func NewTcpConnectProducer(name string, clientFactory *ConnectClientFactory, address string) *TcpConnectProducer {
	p := new(TcpConnectProducer)
	p.name = name
	p.clientFactory = clientFactory
	p.address = address
	p.closeD = chanutil.NewDoneChan()
	p.Logger = golog.New(name)
	return p
}
