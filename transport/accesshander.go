// Copyright 2016 zxfonline@sina.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package transport

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/zxfonline/gerror"
	"github.com/zxfonline/golog"
	"github.com/zxfonline/net/nbtcp"
	"github.com/zxfonline/taskexcutor"
	"github.com/zxfonline/timer"
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
func InitDataAccessHandler(port int32, actimer *timer.Timer) {
	DataAccessHandler = NewAccessHandler(port, actimer)
}
func NewAccessHandler(port int32, actimer *timer.Timer) *AccessHandler {
	if actimer == nil {
		actimer = timer.GTimer
	}
	h := new(AccessHandler)
	h.Logger = golog.New("DataAccessHandler")
	h.entryMap = make(map[int64]entry)
	h.port = port
	h.actimer = actimer
	//收到代理消息回包处理方法
	h.Handler = func(session nbtcp.IoSession, data nbtcp.IoBuffer) {
		mid := data.ReadInt64()
		port := data.ReadInt32()
		pb := NewBuffer(port, data.Bytes())
		h.lock.RLock()
		if e, ok := h.entryMap[mid]; ok {
			h.lock.RUnlock()
			if mid > 0 {
				select {
				case e.waitChan <- pb:
				default:
				}
			} else if mid < 0 {
				h.lock.Lock()
				delete(h.entryMap, mid)
				h.lock.Unlock()
				if e.data == nil {
					return
				}
				pb.SetConnectID(e.data.ConnectID())
				var err error
				pb, err = e.parseData(pb)
				if e.event != nil {
					e.event.Close()
				}
				defer e.clear()
				e.callback.AddArgs(-1, pb, err)
				e.callback.Call(h.actimer.Logger)
			}
		} else {
			h.lock.RUnlock()
		}
	}
	return h
}

//代理消息阻塞访问处理器(接收端使用ProxyCallBackHandler子类来处理消息)
type AccessHandler struct {
	SafeTransmitHandler
	port     int32
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
func (h *AccessHandler) SyncAccess(proxySession nbtcp.IoSession, data nbtcp.IoBuffer, timeout time.Duration) (nbtcp.IoBuffer, error) {
	if timeout == 0 {
		timeout = DEFAULT_ACCESS_TIMEOUT
	}
	return newSyncEntry(proxySession, data, h).syncAccess(timeout)
}

/* 向指定连接的服务进行数据访问,异步请求方式(tcp消息代理)
 *超时后如果没有回执消息抛出通讯超时异常
 * proxySession:远程连接
 * data:代理连接处理的消息包
 * callback:收到消息或者请求超时回调函数，默认最后两个参数顺序为(...,nbtcp.IoBuffer, error),构建时传入的参数顺序不变
 *	ls := len(params)
*	var err error
*	var data nbtcp.IoBuffer
*	if params[ls-1] != nil {
*		err = params[ls-1].(error)
*	} else if params[ls-2] != nil {
*		data = params[ls-2].(nbtcp.IoBuffer)
*	}
 * timeout:访问代理请求超时时间(当传入0时表示使用默认时间 DEFAULT_ACCESS_TIMEOUT)
*/
func (h *AccessHandler) AsyncAccess(proxySession nbtcp.IoSession, data nbtcp.IoBuffer, callback *taskexcutor.TaskService, timeout time.Duration) {
	if timeout == 0 {
		timeout = DEFAULT_ACCESS_TIMEOUT
	}
	newAsyncEntry(proxySession, data, h, callback).asyncAccess(timeout)
}

func (h *AccessHandler) Port() int32 {
	return h.port
}

//同步消息唯一id生成器
var muid int64

//异步消息唯一id生成器
var amuid int64

//构建连接唯一id
func createMId() int64 {
	return atomic.AddInt64(&muid, 1)
}

//构建连接唯一id
func createAmId() int64 {
	return atomic.AddInt64(&amuid, -1)
}

type entry struct {
	//唯一消息号(正数表示同步请求消息号，负数表示异步请求消息号)
	mid int64
	//消息所有者
	session nbtcp.IoSession
	//发送的消息
	data     nbtcp.IoBuffer
	h        *AccessHandler
	waitChan chan nbtcp.IoBuffer
	callback *taskexcutor.TaskService
	event    *timer.TimerEvent
}

func newSyncEntry(session nbtcp.IoSession, data nbtcp.IoBuffer, h *AccessHandler) *entry {
	return &entry{
		h:        h,
		mid:      createMId(),
		session:  session,
		data:     data,
		waitChan: make(chan nbtcp.IoBuffer, 1),
	}
}
func newAsyncEntry(session nbtcp.IoSession, data nbtcp.IoBuffer, h *AccessHandler, callback *taskexcutor.TaskService) *entry {
	return &entry{
		h:        h,
		mid:      createAmId(),
		session:  session,
		data:     data,
		callback: callback,
	}
}

func (e *entry) clear() {
	e.h = nil
	e.data = nil
	e.session = nil
	if e.callback != nil {
		e.callback = nil
	}
	if e.waitChan != nil {
		close(e.waitChan)
	}
}

//代理消息阻塞发送，收到消息或者请求超时返回
func (e *entry) syncAccess(timeout time.Duration) (bb nbtcp.IoBuffer, err error) {
	h := e.h
	out := NewCapBuffer(e.data.Port(), 12+e.data.Len())
	out.Cache(true)
	out.WriteInt64(e.mid)
	out.WriteInt32(h.Port())
	out.WriteBuffer(e.data)
	h.lock.Lock()
	h.entryMap[e.mid] = *e
	h.lock.Unlock()
	e.session.Write(out)
	defer func() {
		h.lock.Lock()
		delete(h.entryMap, e.mid)
		h.lock.Unlock()
		e.clear()
	}()
	select {
	case bb = <-e.waitChan:
		bb.SetConnectID(e.data.ConnectID())
		bb, err = e.parseData(bb)
	case <-time.After(timeout):
		if e.session.Closed() {
			err = ACCESS_IO_ERROR
		} else {
			err = ACCESS_TIMEOUT_ERROR
		}
	}
	return
}
func (e *entry) parseData(data nbtcp.IoBuffer) (nbtcp.IoBuffer, error) {
	errorType := data.ReadInt32()
	if errorType == gerror.OK {
		return data, nil
	}
	detail := data.ReadStr()
	return nil, gerror.NewError(errorType, detail)
}

//代理消息异步发送，收到消息或者请求超时返回
func (e *entry) asyncAccess(timeout time.Duration) {
	h := e.h
	out := NewCapBuffer(e.data.Port(), 12+e.data.Len())
	out.Cache(true)
	out.WriteInt64(e.mid)
	out.WriteInt32(h.Port())
	out.WriteBuffer(e.data)
	h.lock.Lock()
	h.entryMap[e.mid] = *e
	h.lock.Unlock()
	e.session.Write(out)
	//添加超时事件
	e.event = h.actimer.AddOnceEvent(taskexcutor.NewTaskService(func(params ...interface{}) {
		e := (params[1]).(*entry)
		h := e.h
		if h == nil {
			return
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
		defer e.clear()
		e.callback.AddArgs(-1, nil, err)
		e.callback.Call(h.actimer.Logger)
	}, e), "", timeout)
}
