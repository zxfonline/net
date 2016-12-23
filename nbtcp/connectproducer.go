// Copyright 2016 zxfonline@sina.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package nbtcp

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zxfonline/chanutil"
	"github.com/zxfonline/golog"
	. "github.com/zxfonline/net/packet"
	"github.com/zxfonline/taskexcutor"
	"github.com/zxfonline/timer"
)

type TcpConnectProducer struct {
	name          string
	clientFactory *ConnectClientFactory
	address       string
	closeD        chanutil.DoneChan

	event     *timer.TimerEvent
	iosession IoSession
	mu        sync.Mutex
	openning  int32
	trigger   *taskexcutor.TaskService
	Logger    *golog.Logger
}

func (p *TcpConnectProducer) Start(tm *timer.Timer, collateTime time.Duration, trigger *taskexcutor.TaskService) {
	if p.Closed() {
		return
	}
	if trigger == nil {
		trigger = taskexcutor.NewTaskService(p.Ontime)
	}
	trigger.Call(p.Logger)

	p.event = tm.AddTimerEvent(trigger, p.name, collateTime, collateTime, 0, false)
}

func (p *TcpConnectProducer) Ontime(params ...interface{}) {
	if p.clientFactory == nil {
		return
	}
	if p.iosession != nil && !p.iosession.Closed() {
		return
	}
	if p.Closed() {
		return
	}
	p.mu.Lock()
	if atomic.LoadInt32(&p.openning) == 1 { //当前正在建立连接中
		p.mu.Unlock()
		return
	}
	atomic.StoreInt32(&p.openning, 1)
	p.mu.Unlock()
	defer atomic.StoreInt32(&p.openning, 0)
	nio := p.clientFactory.GetConnect(p.address)
	if nio != nil {
		p.iosession = nio
	}
}

func (p *TcpConnectProducer) GetConnect() IoSession {
	return p.iosession
}

//返回本地网络地址
func (p *TcpConnectProducer) LocalAddr() net.Addr {
	if p.iosession == nil {
		return nil
	}
	return p.iosession.LocalAddr()
}

//返回远程网络地址
func (p *TcpConnectProducer) RemoteAddr() net.Addr {
	if p.iosession == nil {
		return nil
	}
	return p.iosession.RemoteAddr()
}

func (p *TcpConnectProducer) Closed() bool {
	return p.closeD.R().Done()
}

func (p *TcpConnectProducer) Close() {
	if p.Closed() {
		return
	}
	p.closeD.SetDone()
	p.event.Close()
	if p.iosession != nil {
		p.iosession.Close()
	}
}
