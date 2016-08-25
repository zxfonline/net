// Copyright 2016 zxfonline@sina.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package transport

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"sync"

	"github.com/zxfonline/timefix"

	"github.com/zxfonline/buffpool"
	"github.com/zxfonline/gerror"
	"github.com/zxfonline/net/nbtcp"
)

var (
	ErrMsgTooBig = gerror.NewError(gerror.SERVER_CDATA_ERROR, "message is too big")
	ErrMsgPacket = gerror.NewError(gerror.SERVER_CDATA_ERROR, "message packet is wrong")
	ErrPackMsg   = gerror.NewError(gerror.SERVER_CMSG_ERROR, "pack message failed")
)

//消息传送格式:消息包头(length) + 消息包类型(type) + 消息体(data)
type MsgRWIO struct {
	RW     io.ReadWriter
	MsgMax uint
}

func NewMsgRWIO(rw io.ReadWriter, msgMax uint) *MsgRWIO {
	return &MsgRWIO{RW: rw, MsgMax: msgMax}
}

//nbtcp.MsgReader.ReadMsg()
func (rw *MsgRWIO) ReadMsg() (data nbtcp.IoBuffer) {
	var _l int32
	err := binary.Read(rw.RW, DefaultEndian, &_l)
	if err != nil {
		panic(err)
	}
	l := int(_l)
	if l < 4 {
		panic(ErrMsgPacket)
	}
	l -= 4
	if rw.MsgMax != 0 && uint(l) > rw.MsgMax {
		panic(ErrMsgTooBig)
	}
	var t int32
	err = binary.Read(rw.RW, DefaultEndian, &t)
	if err != nil {
		panic(err)
	}
	m := buffpool.BufGet(l)
	readed := 0
	var n int
	for readed < l {
		n, err = rw.RW.Read(m[readed:])
		if n > 0 {
			readed += n
			continue
		}
		if err != nil {
			panic(err)
		}
	}
	data = NewBuffer(t, m)
	data.SetRcvPort(t)
	data.SetRcvt(timefix.MillisTime())
	return
}

//nbtcp.MsgWriter.WriteMsg()
func (rw *MsgRWIO) WriteMsg(data nbtcp.IoBuffer) {
	data.SetPost(timefix.MillisTime())
	m := data.Bytes()
	t := data.Port()
	l := len(m)
	//	if rw.MsgMax != 0 && l > rw.MsgMax {
	//		panic(ErrMsgTooBig)
	//	}
	l += 4
	err := binary.Write(rw.RW, DefaultEndian, int32(l))
	if err != nil {
		panic(err)
	}
	err = binary.Write(rw.RW, DefaultEndian, t)
	if err != nil {
		panic(err)
	}
	var n int
	n, err = rw.RW.Write(m)
	if err != nil {
		panic(err)
	}
	if (n + 4) != l {
		panic(ErrPackMsg)
	}
	if data.Cached() {
		buffpool.BufPut(m)
	}
}

//消息输出追踪读写器
type MsgRWDump struct {
	rw        nbtcp.MsgReadWriter
	careAbout func(nbtcp.IoBuffer) bool
	locker    sync.Mutex
	wc        io.WriteCloser
}

func NewMsgRWDump(rw nbtcp.MsgReadWriter, careAbout func(nbtcp.IoBuffer) bool) *MsgRWDump {
	return &MsgRWDump{rw: rw, careAbout: careAbout}
}

func (rw *MsgRWDump) SetDump(wc io.WriteCloser) io.WriteCloser {
	rw.locker.Lock()
	defer rw.locker.Unlock()
	od := rw.wc
	rw.wc = wc
	return od
}

func (rw *MsgRWDump) Dump() io.WriteCloser {
	rw.locker.Lock()
	defer rw.locker.Unlock()
	return rw.wc
}

//nbtcp.StopNotifier.Close()
func (rw *MsgRWDump) Close() {
	rw.locker.Lock()
	defer rw.locker.Unlock()
	if rw.wc != nil {
		rw.wc.Close()
		rw.wc = nil
	}
}

func (rw *MsgRWDump) needDump(data nbtcp.IoBuffer) bool {
	if rw.careAbout != nil {
		return rw.careAbout(data)
	}
	return true
}

//nbtcp.MsgReader.ReadMsg()
func (rw *MsgRWDump) ReadMsg() (data nbtcp.IoBuffer) {
	data = rw.rw.ReadMsg()
	if !rw.needDump(data) {
		return
	}
	if rw.wc == nil {
		return
	}
	rw.locker.Lock()
	defer rw.locker.Unlock()
	m := data.Bytes()
	fmt.Fprintf(rw.wc, "R req:%v len:%v\n", data.Port(), len(m))
	dumper := hex.Dumper(rw.wc)
	dumper.Write(m)
	dumper.Close()
	fmt.Fprintf(rw.wc, "\n\n")
	return
}

//nbtcp.MsgWriter.WriteMsg()
func (rw *MsgRWDump) WriteMsg(data nbtcp.IoBuffer) {
	defer rw.rw.WriteMsg(data)
	if !rw.needDump(data) {
		return
	}
	if rw.wc == nil {
		return
	}
	rw.locker.Lock()
	defer rw.locker.Unlock()
	m := data.Bytes()
	fmt.Fprintf(rw.wc, "W len:%v  %v\n", len(m), data)

	dumper := hex.Dumper(rw.wc)
	dumper.Write(m)
	dumper.Close()
	fmt.Fprintf(rw.wc, "\n\n")
}
