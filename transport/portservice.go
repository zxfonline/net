// Copyright 2016 zxfonline@sina.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transport

import (
	"errors"
	"fmt"

	"github.com/zxfonline/expvar"
	"github.com/zxfonline/gerror"
	"github.com/zxfonline/golog"
	"github.com/zxfonline/iptable"
	. "github.com/zxfonline/net/packet"
	"github.com/zxfonline/trace"
)

//创建消息处理派发器
func NewPortService(name string) *PortService {
	return &PortService{
		cache:   make(map[PackApi]MsgHandler),
		forbid:  make(map[PackApi]bool),
		Logger:  golog.New(name),
		counter: expvar.NewMap(name),
	}
}

//消息处理派发器
type PortService struct {
	cache   map[PackApi]MsgHandler
	forbid  map[PackApi]bool
	Logger  *golog.Logger
	counter *expvar.Map
}

//处理消息通过消息类型进行派发
func (p *PortService) Transmit(session IoSession, data IoBuffer) {
	if h, ok := p.cache[data.Port()]; ok {
		if forbided := p.forbid[data.Port()]; forbided {
			if !iptable.IsTrustedIP1(session.RemoteIP()) {
				panic(gerror.NewError(gerror.SERVER_CDATA_ERROR, fmt.Sprintf("transmit forbid handler,ip:%s,port:%d", session.RemoteIP(), data.Port())))
				return
			}
		}
		if trace.EnableTracing {
			p.counter.Add(data.Port().String(), 1)
		}
		h.Transmit(session, data)
		return
	}
	panic(gerror.NewError(gerror.SERVER_CDATA_ERROR, fmt.Sprintf("transmit no handler,ip:%s,port:%d", session.RemoteIP(), data.Port())))
}

//注册消息处理器 参数错误或者重复注册将触发panic
func (p *PortService) RegistHandler(port PackApi, handler MsgHandler, forbid bool) {
	if handler == nil {
		panic(errors.New("illegal handler error"))
	}
	if _, ok := p.cache[port]; ok {
		panic(fmt.Errorf("repeat regist handler error,port=%d handler=%+v", port.Value(), handler))
	}
	p.cache[port] = handler
	p.forbid[port] = forbid
	p.Logger.Infof("Regist port=%d handler=%+v", port.Value(), handler)
}
