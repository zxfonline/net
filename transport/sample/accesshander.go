package main

import (
	"log"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zxfonline/timer"

	"github.com/zxfonline/fileutil"
	"github.com/zxfonline/gerror"
	. "github.com/zxfonline/net/packet"
	"github.com/zxfonline/taskexcutor"
)

func randInt(min int32, max int32) int32 {
	rand.Seed(time.Now().UTC().UnixNano())
	return min + rand.Int31n(max-min)
}

var ACCESS_TIMEOUT = gerror.NewError(gerror.CLIENT_TIMEOUT, "proxy access timeout")

var DataAccessHandler = NewAccessHandler(ACCESS_RETURN_PORT, nil)
var Session chan IoBuffer

var logger *log.Logger

func main2() {
	Session = make(chan IoBuffer, 10000)
	if wc, err := fileutil.OpenFile("./trace.txt", os.O_TRUNC|os.O_CREATE|os.O_WRONLY, fileutil.DefaultFileMode); err == nil {
		logger = log.New(wc, "", 0)
	}
	logger.Println("start")
	wg := &sync.WaitGroup{}
	wg.Add(1)
	start := time.Now()
	defer func(start time.Time) {
		logger.Printf("over total cost=%s, start=%s,end=%s\n", time.Now().Sub(start), start, time.Now())
	}(start)

	go func() {
		for rbb := range Session { //收到代理回报
			DataAccessHandler.Transmit(rbb)
		}
	}()
	runType := 1
	if runType == 1 {
		//1 async access
		tt := 100
		for i := 1; i <= 100; i++ {
			wg.Add(tt)
			//		go func(i int) {
			for j := 0; j < tt; j++ {
				proxyBf := NewCapBuffer(MsgType(i), 4)
				proxyBf.WriteInt32(int32(j))
				DataAccessHandler.AsyncAccess(proxyBf, taskexcutor.NewTaskService(func(params ...interface{}) {
					i := (params[0]).(int)
					j := (params[1]).(int)
					wg := (params[2]).(*sync.WaitGroup)
					ls := len(params)
					var err error
					var data IoBuffer
					if params[ls-1] != nil {
						err = params[ls-1].(error)
					} else if params[ls-2] != nil {
						data = params[ls-2].(IoBuffer)
					}
					start := params[3].(time.Time)
					logger.Printf("over access cost=%s, start=%s,end=%s callback return i=%v,j=%v,data=%v,err=%v\n", time.Now().Sub(start), start, time.Now(), i, j, data, err)

					if err != nil { //请求错误，后续逻辑

					} else { //请求成功，后续逻辑

					}
					wg.Done()
				}, i, j, wg), 15*time.Second)
			}
			//		}(i)
		}
	} else {

		//			2 sync access
		tt := 100
		for i := 1; i <= 10; i++ {
			wg.Add(1)
			go func(i int) {
				for j := 0; j < tt; j++ {
					proxyBf := NewCapBuffer(MsgType(i), 4)
					proxyBf.WriteInt32(int32(j))
					DataAccessHandler.SyncAccess(proxyBf, 15*time.Second)
				}
				wg.Done()
			}(i)
		}
	}

	wg.Done()
	wg.Wait()
	logger.Println("over")
}

func NewAccessHandler(port MsgType, actimer *timer.Timer) *AccessHandler {
	if actimer == nil {
		actimer = timer.GTimer()
	}
	h := new(AccessHandler)
	h.entryMap = make(map[int64]entry)
	h.port = port
	h.actimer = actimer
	return h
}

type AccessHandler struct {
	port     MsgType
	lock     sync.RWMutex
	actimer  *timer.Timer
	entryMap map[int64]entry
}

func (h *AccessHandler) Transmit(data IoBuffer) {
	mid := data.ReadInt64()
	rqport := MsgType(data.ReadInt32())
	pb := NewBuffer(rqport, data.Bytes())
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
			if e.callback == nil {
				return
			}
			pb.SetConnectID(e.connId)
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
		logger.Printf("超时后收到回包，不处理回包。data=%d,%+v\n", pb.Port(), pb)
	}
}

func (h *AccessHandler) SyncAccess(data IoBuffer, timeout time.Duration) (rb IoBuffer, err error) {
	start := time.Now()
	defer func(start time.Time) {
		logger.Printf("over access cost=%s, start=%s,end=%s block return value=%+v,err=%v\n", time.Now().Sub(start), start, time.Now(), rb, err)
	}(start)
	rb, err = newSyncEntry(data.ConnectID(), h).syncAccess(data, timeout)
	return
}

func (h *AccessHandler) AsyncAccess(data IoBuffer, callback *taskexcutor.TaskService, timeout time.Duration) {
	newAsyncEntry(data.ConnectID(), h, callback).asyncAccess(data, timeout)
}

func (h *AccessHandler) Port() MsgType {
	return h.port
}

//消息唯一id生成器
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
	h        *AccessHandler
	waitChan chan IoBuffer
	//唯一消息号(正数表示同步请求消息号，负数表示异步请求消息号)
	mid      int64
	connId   int64
	callback *taskexcutor.TaskService
	event    *timer.TimerEvent
}

func newSyncEntry(connId int64, h *AccessHandler) *entry {
	return &entry{
		h:        h,
		mid:      createMId(),
		connId:   connId,
		waitChan: make(chan IoBuffer, 1),
	}
}
func newAsyncEntry(connId int64, h *AccessHandler, callback *taskexcutor.TaskService) *entry {
	return &entry{
		h:        h,
		mid:      createAmId(),
		connId:   connId,
		callback: callback,
	}
}
func (e *entry) clear() {
	e.h = nil
	if e.callback != nil {
		e.callback = nil
	}
	if e.waitChan != nil {
		close(e.waitChan)
	}
}

//网络代理发送数据
func (e *entry) syncAccess(data IoBuffer, timeout time.Duration) (bb IoBuffer, err error) {
	h := e.h
	out := NewCapBuffer(data.Port(), 12+data.Len())
	out.WriteInt64(e.mid)
	out.WriteInt32(h.Port().Value())
	out.WriteBuffer(data)
	h.lock.Lock()
	h.entryMap[e.mid] = *e
	h.lock.Unlock()

	go func(in IoBuffer) {
		//模仿网络通信,接收端收到消息
		mid := in.ReadInt64()
		rtport := MsgType(in.ReadInt32())
		//		sl := randInt(0.5e3, 2e3)
		//		fmt.Printf("receive data=%d,sleep=%+v\n", mid, sl)
		//		time.Sleep(time.Duration(sl) * time.Millisecond)
		wcap := 16
		rbb := NewCapBuffer(rtport, wcap)
		rbb.WriteInt64(mid)
		rbb.WriteInt32(in.Port().Value())
		//1
		rbb.WriteInt32(gerror.OK.Value())
		//2
		//		ae := gerror.NewError(gerror.SERVER_ACCESS_REFUSED, "服务器拒绝该项访问")
		//		rbb.WriteInt32(ae.Code)
		//		rbb.WriteString(ae.Content)
		Session <- rbb
	}(out)
	defer func() {
		h.lock.Lock()
		delete(h.entryMap, e.mid)
		h.lock.Unlock()
		e.clear()
	}()
	select {
	case bb = <-e.waitChan:
		bb.SetConnectID(e.connId)
		bb, err = e.parseData(bb)
		//		fmt.Printf("收到回包，data=%+v,%v,entry=%+v\n", bb, err, e)
	case <-time.After(timeout):
		err = ACCESS_TIMEOUT
		logger.Printf("代理请求超时了 entry=%+v\n", e.mid)
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
	start := time.Now()
	h := e.h
	out := NewCapBuffer(data.Port(), 12+data.Len())
	out.WriteInt64(e.mid)
	out.WriteInt32(h.Port().Value())
	out.WriteBuffer(data)
	h.lock.Lock()
	h.entryMap[e.mid] = *e
	h.lock.Unlock()
	//添加超时事件
	e.callback.AddArgs(-1, start)
	e.event = h.actimer.AddOnceEvent(taskexcutor.NewTaskService(func(params ...interface{}) {
		e := (params[1]).(*entry)
		h := e.h
		if h == nil {
			return
		}
		h.lock.Lock()
		delete(h.entryMap, e.mid)
		h.lock.Unlock()
		defer e.clear()
		e.callback.AddArgs(-1, nil, ACCESS_TIMEOUT)
		e.callback.Call(h.actimer.Logger)
	}, e), "", timeout)

	go func(in IoBuffer) {
		//模仿网络通信,接收端收到消息
		mid := in.ReadInt64()
		rtport := MsgType(in.ReadInt32())

		sl := randInt(0.5e3, 2e3)
		//		fmt.Printf("receive data=%d,sleep=%+v\n", mid, sl)
		time.Sleep(time.Duration(sl) * time.Millisecond)
		wcap := 16
		rbb := NewCapBuffer(rtport, wcap)
		rbb.WriteInt64(mid)
		rbb.WriteInt32(in.Port().Value())
		//1
		rbb.WriteInt32(gerror.OK.Value())
		//2
		//		ae := gerror.NewError(gerror.SERVER_ACCESS_REFUSED, "服务器拒绝该项访问")
		//		rbb.WriteInt32(ae.Code)
		//		rbb.WriteString(ae.Content)
		Session <- rbb
	}(out)
}
