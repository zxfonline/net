// Copyright 2016 zxfonline@sina.com. All rights reserved.
// Use of s source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nbtcp

import (
	"net"
	"sync"
	"sync/atomic"

	"io"
	"time"

	"context"

	"github.com/zxfonline/chanutil"
	. "github.com/zxfonline/net/conn"
	. "github.com/zxfonline/net/packet"
	"github.com/zxfonline/taskexcutor"
	"golang.org/x/net/trace"
)

//服务器结构体
type Server struct {
	//tcp连接唯一id生成器
	cuid int64
	//服务器配置
	config  *ServerConfig
	address string
	//服务器开启的线程组等待列表
	wg       *sync.WaitGroup
	stopOnce sync.Once
	//消息处理器
	msgProcessor MsgHandler
	//解码器
	ioc func(io.ReadWriter, IoSession) MsgReadWriter
	//过滤器
	iofilterRegister func(IoSession) IoFilterChain
	l                net.Listener
	quitF            context.CancelFunc
	stopD            chanutil.DoneChan
	events           trace.EventLog
}

//构建连接唯一id
func (s *Server) createConnectId() int64 {
	return atomic.AddInt64(&s.cuid, 1)
}
func (s *Server) working(ctx context.Context, msgExcutor taskexcutor.Excutor, mutilMsg bool) {
	defer func(ctx context.Context) {
		if !s.Closed() {
			if e := recover(); e != nil {
				serverLogger.Errorf("recover error:%v", e)
				s.TraceErrorf("working err:%v", e)
			}
			//尝试重连
			err := s.startListener(MAXN_RETRY_TIMES)
			if err != nil { //重连失败
				s.Close()
			} else { //重连成功，继续工作
				go s.working(ctx, msgExcutor, mutilMsg)
			}
		} else {
			if e := recover(); e != nil {
				serverLogger.Debugf("recover error:%v", e)
			}
		}
	}(ctx)
	s.accept(ctx, msgExcutor, mutilMsg)
}
func (s *Server) accept(ctx context.Context, msgExcutor taskexcutor.Excutor, mutilMsg bool) {
	var tempDelay time.Duration
	for q := false; !q; {
		c, err := s.l.Accept()
		if err != nil {
			s.TraceErrorf("done serving; Accept = %v", err)
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				serverLogger.Errorf("tcp: Accept error:%s.retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			panic(err)
		}
		tempDelay = 0
		s.newClient(ctx, c, msgExcutor, mutilMsg)
	}
}

func (s *Server) startListener(trytime int) error {
	trytime--
	//	err := s.listener()
	err := func() error {
		if s.l != nil {
			s.l.Close()
			s.l = nil
		}
		l, err := net.Listen("tcp", s.address)
		if err != nil {
			return err
		}
		s.l = l
		serverLogger.Infof("tcp serving %s", l.Addr())
		s.TracePrintf("serving")
		return nil
	}()
	if err != nil {
		if trytime > 0 {
			time.Sleep(1 * time.Second)
			serverLogger.Errorf("tcp: Listen error:%s; retrying %d", err, trytime+1)
			return s.startListener(trytime)
		} else {
			return err
		}
	}
	return nil
}

//启动服务器
func (s *Server) Start(msgExcutor taskexcutor.Excutor, mutilMsg bool) {
	err := s.startListener(1)
	if err != nil {
		panic(err)
	}
	s.wg.Add(1)
	var ctx context.Context
	ctx, s.quitF = context.WithCancel(context.Background())
	go s.working(ctx, msgExcutor, mutilMsg)
}

func (s *Server) newClient(ctx context.Context, conn net.Conn, msgExcutor taskexcutor.Excutor, mutilMsg bool) {
	c := CreateConnect(s.wg, conn, s.config.ChanReadSize, s.config.ChanSendSize, s.config.DeadlineSecond, s.config.ReadBufMaxSize)
	c.Open(ctx, msgExcutor, s.createConnectId(), s.msgProcessor, s.ioc, s.iofilterRegister, mutilMsg)
}

//关闭服务器 shutdown.StopNotifier.Close()
func (s *Server) Close() {
	s.stopOnce.Do(func() {
		serverLogger.Infof("STOPING %+v", s)
		s.stopD.SetDone()
		s.l.Close()
		s.l = nil
		s.quitF()
		serverLogger.Infof("STOPED %+v", s)
		s.TracePrintf("server closed")
		s.wg.Done()
		if s.events != nil {
			s.events.Finish()
			s.events = nil
		}
	})
}

//是否关闭
func (s *Server) Closed() bool {
	return s.stopD.R().Done()
}

// printf records an event in s's event log, unless s has been stopped.
func (s *Server) TracePrintf(format string, a ...interface{}) {
	if s.events != nil {
		s.events.Printf(format, a...)
	}
}

// errorf records an error in s's event log, unless s has been stopped.
func (s *Server) TraceErrorf(format string, a ...interface{}) {
	if s.events != nil {
		s.events.Errorf(format, a...)
	}
}
