// Copyright 2016 zxfonline@sina.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package transport

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zxfonline/timefix"

	. "github.com/zxfonline/net/packet"

	"github.com/zxfonline/gerror"
	"github.com/zxfonline/golog"
	"github.com/zxfonline/taskexcutor"
	"github.com/zxfonline/timer"
	. "github.com/zxfonline/trace"
	"golang.org/x/net/trace"
)

var (
	ACCESS_TIMEOUT_ERROR = gerror.NewError(gerror.CLIENT_TIMEOUT, "proxy access timeout")
	ACCESS_IO_ERROR      = gerror.NewError(gerror.CLIENT_IO_ERROR, "proxy access io error")
)

const (
	//默认代理访问超时时间10s
	DEFAULT_ACCESS_TIMEOUT = 10 * time.Second
)

//代理访问消息接口注册
var DataAccessHandler *AccessHandler

//初始消息代理访问接口
func InitDataAccessHandler(port MsgType, actimer *timer.Timer) {
	DataAccessHandler = NewAccessHandler(port, actimer)
}
func NewAccessHandler(port MsgType, actimer *timer.Timer) *AccessHandler {
	if actimer == nil {
		actimer = timer.GTimer()
	}
	h := new(AccessHandler)
	h.Logger = golog.New(port.String())
	h.entryMap = make(map[int64]entry)
	h.port = port
	h.actimer = actimer
	//收到代理消息回包处理方法
	h.Handler = func(session IoSession, data IoBuffer, logger *golog.Logger) {
		mid := data.ReadInt64()
		port := MsgType(data.ReadInt32())
		h.lock.RLock()
		if e, ok := h.entryMap[mid]; ok {
			h.lock.RUnlock()
			if mid > 0 {
				pb := NewBuffer(port, data.Bytes())
				pb.SetRcvPort(port)
				pb.SetRcvt(data.GetRcvt())
				pb.SetPrct(data.GetPrct())
				select {
				case e.waitChan <- pb:
				default:
				}
				data.TracePrintf("sync proxy callback post ok, port:%d, mid:%d", port, mid)
			} else if mid < 0 {
				h.lock.Lock()
				delete(h.entryMap, mid)
				h.lock.Unlock()
				if e.session == nil {
					data.TraceErrorf("async proxy callback timeout, port:%d, mid:%d", port, mid)
					return
				} else {
					data.TracePrintf("async proxy callback, port:%d, mid:%d", port, mid)
				}
				var err error
				_, err = e.parseData(data)
				if e.event != nil {
					e.event.Close()
				}
				defer e.clear()
				e.callback.AddArgs(-1, data, err)
				e.callback.Call(h.actimer.Logger)
			}
		} else {
			if mid > 0 {
				data.TraceErrorf("sync proxy callback timeout, port:%d, mid:%d", port, mid)
			} else {
				data.TraceErrorf("async proxy callback timeout, port:%d, mid:%d", port, mid)
			}
			h.lock.RUnlock()
		}
	}
	return h
}

//代理消息阻塞访问处理器(接收端使用ProxyCallBackHandler子类来处理消息)
type AccessHandler struct {
	SafeTransmitHandler
	port     MsgType
	lock     sync.RWMutex
	actimer  *timer.Timer
	entryMap map[int64]entry
}

/* 向指定连接的服务进行数据访问,当前线程休眠设置的时间,然后继续处理（一般用于http代理请求，tcp消息代理尽量使用异步请求方式AsyncAccess）
 *在休眠时间内主要是等待远程服务器的回执消息,如果没有回执消息抛出通讯超时异常
 * 注意:
 * 服务器之间的通讯如果是使用线程池中的线程,尽量不要使用该类型通讯方式,特别是当服务器之间出现大量的该类通讯方式,
 * 会将线程池中的可用线程耗尽,直到线程池缓存任务队列满之前,将不会有新线程来处理
 * 队列中的任务,尽量选用异步数据访问
 * proxySession:远程连接
 * data:代理连接处理的消息包
 * timeout:访问代理请求超时时间(当传入0时表示使用默认时间 DEFAULT_ACCESS_TIMEOUT)
 */
func (h *AccessHandler) SyncAccess(proxySession IoSession, data IoBuffer, timeout time.Duration) (IoBuffer, error) {
	if timeout == 0 {
		timeout = DEFAULT_ACCESS_TIMEOUT
	}
	return newSyncEntry(proxySession, data.ConnectID(), h).syncAccess(data, timeout)
}

/* 向指定连接的服务进行数据访问,异步请求方式(tcp消息代理)
 *超时后如果没有回执消息抛出通讯超时异常
 * proxySession:远程连接
 * data:代理连接处理的消息包
 * callback:收到消息或者请求超时回调函数，默认最后两个参数顺序为(...,data IoBuffer, err error),构建时传入的参数顺序不变
 *	ls := len(params)
*	var err error
*	var data IoBuffer
*	if params[ls-1] != nil {
*		err = params[ls-1].(error)
*	} else if params[ls-2] != nil {
*		data = params[ls-2].(IoBuffer)
*	}
*	//data 永远不会为空，只能通过err是否为空来判断消息是否成功
*	if err != nil { //请求错误，后续逻辑
*		//TODO ...
*	} else { //请求成功，后续逻辑
*		//TODO ...
*	}
*
*
 * timeout:访问代理请求超时时间(当传入0时表示使用默认时间 DEFAULT_ACCESS_TIMEOUT)
*/
func (h *AccessHandler) AsyncAccess(proxySession IoSession, data IoBuffer, callback *taskexcutor.TaskService, timeout time.Duration) {
	if timeout == 0 {
		timeout = DEFAULT_ACCESS_TIMEOUT
	}
	newAsyncEntry(proxySession, data.ConnectID(), h, callback).asyncAccess(data, timeout)
}

func (h *AccessHandler) Port() MsgType {
	return h.port
}

//同步消息唯一id生成器
var muid int64

//异步消息唯一id生成器
var amuid int64

//构建消息唯一id 正数
func createMId() int64 {
	return atomic.AddInt64(&muid, 1)
}

//构建消息唯一id 负数
func createAmId() int64 {
	return atomic.AddInt64(&amuid, -1)
}

type entry struct {
	//唯一消息号(正数表示同步请求消息号，负数表示异步请求消息号)
	mid int64
	//消息所有者
	session  IoSession
	connId   int64
	h        *AccessHandler
	waitChan chan IoBuffer
	callback *taskexcutor.TaskService
	event    *timer.TimerEvent
}

func newSyncEntry(session IoSession, connId int64, h *AccessHandler) *entry {
	return &entry{
		h:        h,
		mid:      createMId(),
		session:  session,
		connId:   connId,
		waitChan: make(chan IoBuffer, 1),
	}
}
func newAsyncEntry(session IoSession, connId int64, h *AccessHandler, callback *taskexcutor.TaskService) *entry {
	return &entry{
		h:        h,
		mid:      createAmId(),
		session:  session,
		connId:   connId,
		callback: callback,
	}
}

func (e *entry) clear() {
	e.h = nil
	e.session = nil
	if e.callback != nil {
		e.callback = nil
	}
	if e.waitChan != nil {
		close(e.waitChan)
	}
}

//代理消息阻塞发送，收到消息或者请求超时返回
func (e *entry) syncAccess(data IoBuffer, timeout time.Duration) (bb IoBuffer, err error) {
	h := e.h
	out := NewCapBuffer(data.Port(), 12+data.Len())
	out.WriteInt64(e.mid)
	out.WriteInt32(h.Port().Value())
	out.WriteBuffer(data)
	h.lock.Lock()
	h.entryMap[e.mid] = *e
	h.lock.Unlock()
	data.TracePrintf("sync proxy, port:%d, mid:%d", data.Port(), e.mid)
	e.session.Write(out)
	defer func() {
		h.lock.Lock()
		delete(h.entryMap, e.mid)
		h.lock.Unlock()
		e.clear()
	}()
	data.Reset()
	bb = data
	select {
	case bb = <-e.waitChan:
		bb.SetConnectID(e.connId)
		bb, err = e.parseData(bb)
		if bb != nil {
			bb.RegistTraceInfo(data.TraceInfo())
			data.TracePrintf("sync proxy callback ok.")
		} else {
			data.TracePrintf("sync proxy callback ok,err:%v", err)
		}
	case <-time.After(timeout):
		if e.session.Closed() {
			err = ACCESS_IO_ERROR
		} else {
			err = ACCESS_TIMEOUT_ERROR
		}
		data.TraceErrorf("sync proxy callback timeout,err:%v", err)
	}
	return
}
func (e *entry) parseData(data IoBuffer) (IoBuffer, error) {
	errorType := gerror.ErrorType(data.ReadInt32())
	if errorType == gerror.OK {
		return data, nil
	}
	detail := data.ReadString()
	return nil, gerror.NewError(errorType, detail)
}

//代理消息异步发送，收到消息或者请求超时返回
func (e *entry) asyncAccess(data IoBuffer, timeout time.Duration) {
	h := e.h
	out := NewCapBuffer(data.Port(), 12+data.Len())
	out.WriteInt64(e.mid)
	out.WriteInt32(h.Port().Value())
	out.WriteBuffer(data)
	h.lock.Lock()
	h.entryMap[e.mid] = *e
	h.lock.Unlock()
	//添加超时事件
	e.event = h.actimer.AddOnceEvent(taskexcutor.NewTaskService(func(params ...interface{}) {
		e := (params[1]).(*entry)
		port := (params[2]).(MsgType)
		h := e.h
		if h == nil {
			return
		}
		data := NewBuffer(port, nil)
		data.SetRcvPort(port)
		data.SetRcvt(timefix.MillisTime())
		if EnableTracing {
			data.RegistTraceInfo(trace.New(fmt.Sprintf("async_callback.port_%d", port), "buffer"))
		}

		h.lock.Lock()
		delete(h.entryMap, e.mid)
		h.lock.Unlock()
		var err error
		if e.session != nil && e.session.Closed() {
			err = ACCESS_IO_ERROR
		} else {
			err = ACCESS_TIMEOUT_ERROR
		}
		data.TraceErrorf("async proxy callback timeout, mid:%d,err:%v", e.mid, err)
		defer e.clear()
		e.callback.AddArgs(-1, data, err)
		e.callback.Call(h.actimer.Logger)
	}, e, data.Port()), "", timeout)
	data.TracePrintf("async proxy, port:%d, mid:%d", data.Port(), e.mid)
	e.session.Write(out)
}
