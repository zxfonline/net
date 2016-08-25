// Copyright 2016 zxfonline@sina.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package nbtcp

import (
	"errors"
	"fmt"

	"github.com/zxfonline/gerror"
	"github.com/zxfonline/golog"
)

//消息处理派发器
type PortService struct {
	cache  map[int32]MsgHandler
	Logger *golog.Logger
}

//处理消息通过消息类型进行派发
func (p *PortService) Transmit(session IoSession, data IoBuffer) {
	if h, ok := p.cache[data.Port()]; ok {
		h.Transmit(session, data)
		return
	}
	panic(gerror.NewError(gerror.SERVER_CDATA_ERROR, fmt.Sprintf("transmit no handler,port=%d", data.Port())))
}

//注册消息处理器 参数错误或者重复注册将触发panic
func (p *PortService) RegistHandler(port int32, handler MsgHandler) {
	if handler == nil {
		panic(errors.New("illegal handler error"))
	}
	if _, ok := p.cache[port]; ok {
		panic(fmt.Errorf("repeat regist handler error,port=%d handler=%+v", port, handler))
	}
	p.cache[port] = handler
	p.Logger.Infof("Regist port=%d handler=%+v", port, handler)
}

//创建消息处理派发器
func NewPortService(name string) *PortService {
	return &PortService{
		cache:  make(map[int32]MsgHandler),
		Logger: golog.New(name),
	}
}
