// Copyright 2016 zxfonline@sina.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package transport

import (
	"time"

	"github.com/zxfonline/net/nbtcp"
	"github.com/zxfonline/taskexcutor"
	"github.com/zxfonline/timer"
)

func NewTimerTcpConnectProducer(name string, clientFactory *nbtcp.ConnectClientFactory, address string) *TimerTcpConnectProducer {
	p := new(TimerTcpConnectProducer)
	p.TcpConnectProducer = *nbtcp.NewTcpConnectProducer(name, clientFactory, address)
	return p
}

type TimerTcpConnectProducer struct {
	nbtcp.TcpConnectProducer
}

//定时器调用方法
func (p *TimerTcpConnectProducer) Ontime(params ...interface{}) {
	p.TcpConnectProducer.Ontime(params)
	connect := p.GetConnect()
	if connect == nil || connect.Closed() {
		return
	}
	//定时获取时间
	connect.Write(NewBuffer(REQ_TIME_PORT, nil))
}

func (p *TimerTcpConnectProducer) TimeFixStart(tm *timer.Timer, collateTime time.Duration) {
	p.TcpConnectProducer.Start(tm, collateTime, taskexcutor.NewTaskService(p.Ontime))
}
