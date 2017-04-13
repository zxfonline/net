// Copyright 2016 zxfonline@sina.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nbtcp

import (
	"sync"
	"sync/atomic"
	"time"

	"io"

	"context"

	"github.com/zxfonline/chanutil"
	. "github.com/zxfonline/net/packet"
	"github.com/zxfonline/taskexcutor"
	. "github.com/zxfonline/trace"
	"golang.org/x/net/trace"
)

//客户端连接工厂,提供创建连接并维护建立的连接
type ConnectClientFactory struct {
	//tcp连接唯一id生成器
	cuid int64
	//服务器配置
	config *ServerConfig
	//服务器开启的线程组等待列表
	wg       *sync.WaitGroup
	stopOnce sync.Once
	//消息处理器
	msgProcessor MsgHandler
	//解码器
	ioc func(io.ReadWriter, IoSession) MsgReadWriter
	//过滤器
	iofilterRegister func(IoSession) IoFilterChain
	quitF            context.CancelFunc
	stopD            chanutil.DoneChan
	//缓存建立的连接
	connectMap  map[string]IoSession
	clock       sync.RWMutex
	crtChan     chan string
	sessionChan chan IoSession
	tr          trace.Trace
}

//构建连接唯一id
func (s *ConnectClientFactory) createConnectId() int64 {
	return atomic.AddInt64(&s.cuid, 1)
}

func (s *ConnectClientFactory) working(ctx context.Context, msgExcutor taskexcutor.Excutor, timeout time.Duration, mutilMsg bool) {
	for q := false; !q; {
		select {
		case <-s.stopD:
			q = true
		case address := <-s.crtChan:
			c := OpenConnect(ctx, address, timeout, s.wg, s.createConnectId(), s.msgProcessor, s.ioc, s.iofilterRegister, msgExcutor, s.config.ChanReadSize, s.config.ChanSendSize, s.config.DeadlineSecond, s.config.ReadBufMaxSize, mutilMsg)
			s.sessionChan <- c
		}
	}
}

//启动 timeout:连接建立请求超时时间
func (s *ConnectClientFactory) Start(msgExcutor taskexcutor.Excutor, timeout time.Duration, mutilMsg bool) {
	s.wg.Add(1)
	var ctx context.Context
	if EnableTracing {
		s.tr = trace.New("connectfactory", "trace")
	}
	ctx, s.quitF = context.WithCancel(context.Background())
	go s.working(ctx, msgExcutor, timeout, mutilMsg)
}

//关闭 shutdown.StopNotifier.Close()
func (s *ConnectClientFactory) Close() {
	s.stopOnce.Do(func() {
		clientFLogger.Infof("STOPING %+v", s)
		s.stopD.SetDone()
		s.quitF()
		clientFLogger.Infof("STOPED %+v", s)
		if s.tr != nil {
			s.tr.Finish()
			s.tr = nil
		}
		s.wg.Done()
	})
}

//是否关闭
func (s *ConnectClientFactory) Closed() bool {
	return s.stopD.R().Done()
}

//获取连接,缓存中有则直接取出，没有新建并并将连接缓存
func (s *ConnectClientFactory) GetConnect(address string) (c IoSession) {
	defer func() {
		if e := recover(); e != nil {
			clientFLogger.Warnf("recover error:%v", e)
			c = nil
		}
	}()
	if s.Closed() {
		clientFLogger.Warnf("GetInstance error,Facotry closed,add=%s", address)
		return
	}
	c = s.checkInstance(address)
	if c != nil {
		return
	}
	s.crtChan <- address
	c = <-s.sessionChan
	if c != nil {
		s.clock.Lock()
		s.connectMap[address] = c
		s.clock.Unlock()
		s.tr.LazyPrintf("openInstance [%v] success", address)
	} else {
		if s.tr != nil {
			s.tr.LazyPrintf("openInstance [%v] fails", address)
			s.tr.SetError()
		}
	}
	return
}

//获取缓存的连接
func (s *ConnectClientFactory) checkInstance(address string) IoSession {
	s.clock.RLock()
	if c, ok := s.connectMap[address]; ok {
		s.clock.RUnlock()
		if c.Closed() {
			s.clock.Lock()
			delete(s.connectMap, address)
			s.clock.Unlock()
			clientFLogger.Debugf("checkInstance,connect closed,%+v", c)
			if s.tr != nil {
				s.tr.LazyPrintf("checkInstance [%v] closed", address)
				s.tr.SetError()
			}
			return nil
		}
		return c
	} else {
		s.clock.RUnlock()
	}
	return nil
}
