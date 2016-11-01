// Copyright 2016 zxfonline@sina.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package transport

import (
	"errors"
	"fmt"

	"github.com/zxfonline/gerror"
	"github.com/zxfonline/golog"
	"github.com/zxfonline/net/nbtcp"
)

//自动回执的消息处理接口
func NewCallBackHandler(logger *golog.Logger, Handler func(nbtcp.IoSession, nbtcp.IoBuffer) (nbtcp.IoBuffer, error)) *CallBackHandler {
	return &CallBackHandler{Logger: logger, Handler: Handler}
}

//自动代理回执的消息处理接口
func NewProxyCallBackHandler(logger *golog.Logger, Handler func(nbtcp.IoSession, nbtcp.IoBuffer) (nbtcp.IoBuffer, error)) *ProxyCallBackHandler {
	return &ProxyCallBackHandler{Logger: logger, Handler: Handler}
}

//安全的消息处理接口
func NewSafeTransmitHandler(logger *golog.Logger, Handler func(iosession nbtcp.IoSession, in nbtcp.IoBuffer)) *SafeTransmitHandler {
	return &SafeTransmitHandler{Logger: logger, Handler: Handler}
}

//客户端异步方式访问的消息处理端口通用类，有回执消息则会返回一个消息包
type CallBackHandler struct {
	Handler func(nbtcp.IoSession, nbtcp.IoBuffer) (nbtcp.IoBuffer, error)
	Logger  *golog.Logger
}

func (h *CallBackHandler) transwork(iosession nbtcp.IoSession, in nbtcp.IoBuffer) (out nbtcp.IoBuffer, err error) {
	defer func() {
		if e := recover(); e != nil {
			switch v := e.(type) {
			case error:
				err = v
			case string:
				err = errors.New(v)
			default:
				err = fmt.Errorf("%v", e)
			}
			in.TraceErrorf("process err:%v", e)
		}
	}()
	out, err = h.Handler(iosession, in)
	return
}

//nbtcp.MsgHandler.Transmit()
func (h *CallBackHandler) Transmit(iosession nbtcp.IoSession, in nbtcp.IoBuffer) {
	out, err := h.transwork(iosession, in)
	if err != nil { //处理消息有错
		panic(err)
	}
	if out != nil { //有回执消息
		in.TracePrintf("process callback")
		h.send(iosession, out)
	}
}

func (h *CallBackHandler) send(iosession nbtcp.IoSession, out nbtcp.IoBuffer) {
	iosession.Write(out)
}

//客户端阻塞方式访问的消息处理端口通用类（AccessHandler），默认会读取消息号和回执端口号，不管处理后是否有回执消息，默认都会将消息号回执回去
type ProxyCallBackHandler struct {
	Handler func(nbtcp.IoSession, nbtcp.IoBuffer) (nbtcp.IoBuffer, error)
	Logger  *golog.Logger
}

func (h *ProxyCallBackHandler) transwork(iosession nbtcp.IoSession, in nbtcp.IoBuffer) (out nbtcp.IoBuffer, err error) {
	defer func() {
		if e := recover(); e != nil {
			switch v := e.(type) {
			case error:
				err = v
			case string:
				err = errors.New(v)
			default:
				err = fmt.Errorf("%v", e)
			}
			in.TraceErrorf("process err:%v", e)
		}
	}()
	out, err = h.Handler(iosession, in)
	return
}

//nbtcp.MsgHandler.Transmit()
func (h *ProxyCallBackHandler) Transmit(iosession nbtcp.IoSession, in nbtcp.IoBuffer) {
	mid := in.ReadInt64()
	rtport := in.ReadInt32()
	rqport := in.Port()
	out, err := h.transwork(iosession, in)
	if err != nil { //处理消息有错
		var gerr *gerror.SysError
		switch v := err.(type) {
		case *gerror.SysError:
			gerr = v
		default:
			gerr = gerror.New(gerror.SERVER_CMSG_ERROR, err)
			h.Logger.Warnf("Proxy Access error:%v", err)
		}
		in.TracePrintf("process callback")
		h.sendError(rqport, mid, rtport, iosession, gerr)
	} else {
		in.TracePrintf("process callback")
		h.send(rqport, mid, rtport, iosession, out)
	}
}

func (h *ProxyCallBackHandler) sendError(rqport int32, mid int64, rtport int32, iosession nbtcp.IoSession, gerr *gerror.SysError) {
	bb := NewCapBuffer(rtport, 16+len(gerr.Content)+2)
	bb.Cache(true)
	bb.WriteInt64(mid)
	bb.WriteInt32(rqport)
	bb.WriteInt32(gerr.Code)
	bb.WriteStr(gerr.Content)
	iosession.Write(bb)
}

func (h *ProxyCallBackHandler) send(rqport int32, mid int64, rtport int32, iosession nbtcp.IoSession, out nbtcp.IoBuffer) {
	wcap := 16
	if out != nil {
		wcap += out.Len()
	}
	bb := NewCapBuffer(rtport, wcap)
	bb.Cache(true)
	bb.WriteInt64(mid)
	bb.WriteInt32(rqport)
	bb.WriteInt32(gerror.OK)
	if out != nil {
		bb.WriteBuffer(out)
	}
	iosession.Write(bb)
}

//安全的消息处理器，捕获的移除，不至于发生异常关闭连接，用于服务器之间不需要自动回执的消息处理（如果需要自动回执的使用ProxyCallBackHandler）
type SafeTransmitHandler struct {
	Handler func(iosession nbtcp.IoSession, in nbtcp.IoBuffer)
	Logger  *golog.Logger
}

//nbtcp.MsgHandler.Transmit()
func (h *SafeTransmitHandler) Transmit(iosession nbtcp.IoSession, in nbtcp.IoBuffer) {
	defer func() {
		if e := recover(); e != nil {
			h.Logger.Warnf("Transmit error:%v", e)
			in.TraceErrorf("process err:%v", e)
		}
	}()
	h.Handler(iosession, in)
}
