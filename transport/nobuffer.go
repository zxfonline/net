// Copyright 2016 zxfonline@sina.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package transport

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"time"

	"github.com/zxfonline/buffpool"
	"github.com/zxfonline/net/nbtcp"
	"github.com/zxfonline/timefix"
)

//默认字节序 大端法
var DefaultEndian = binary.BigEndian

func NewBuffer(port int32, buf []byte) nbtcp.IoBuffer {
	b := &nbuffer{port: port, buf: bytes.NewBuffer(buf)}
	return b
}

func NewCapBuffer(port int32, caps int) nbtcp.IoBuffer {
	buf := buffpool.BufGet(caps)
	buf = buf[:0]
	b := &nbuffer{port: port, buf: bytes.NewBuffer(buf)}
	return b
}

type nbuffer struct {
	port      int32
	rcvPort   int32
	connectId int64
	buf       *bytes.Buffer
	cache     bool
	rcvt      int64 //收到消息的时间
	prct      int64 //开始处理时间
	post      int64 //消息处理完成/发送时间
}

func (nb *nbuffer) GetRcvt() int64 {
	return nb.rcvt
}
func (nb *nbuffer) SetRcvt(rcvt int64) {
	nb.rcvt = rcvt
}
func (nb *nbuffer) GetPrct() int64 {
	return nb.prct
}
func (nb *nbuffer) SetPrct(prct int64) {
	nb.prct = prct
}
func (nb *nbuffer) GetPost() int64 {
	return nb.post
}
func (nb *nbuffer) SetPost(post int64) {
	nb.post = post
}
func (nb *nbuffer) ConnectID() int64 {
	return nb.connectId
}
func (nb *nbuffer) SetConnectID(cid int64) {
	nb.connectId = cid
}

func (nb *nbuffer) Reset() {
	nb.buf.Reset()
}
func (nb *nbuffer) Len() int {
	return nb.buf.Len()
}
func (nb *nbuffer) Bytes() []byte {
	return nb.buf.Bytes()
}
func (nb *nbuffer) Cached() bool {
	return nb.cache
}
func (nb *nbuffer) Cache(cache bool) {
	nb.cache = cache
}

func (nb *nbuffer) Port() int32 {
	return nb.port
}

//消息包复用，一般处理完消息后直接复用该消息包，更改消息传输类型并填充数据回执给请求方
func (nb *nbuffer) SetPort(port int32) {
	nb.port = port
}
func (nb *nbuffer) RcvPort() int32 {
	return nb.rcvPort
}
func (nb *nbuffer) SetRcvPort(rcvPort int32) {
	nb.rcvPort = rcvPort
}

//写入字节数组，不包含了长度头
func (nb *nbuffer) WriteData(bb []byte) {
	err := binary.Write(nb.buf, DefaultEndian, bb)
	if err != nil {
		panic(err)
	}
}

//写入buffer中未读的字节，不包含了长度头
func (nb *nbuffer) WriteBuffer(io nbtcp.IoBuffer) {
	nb.WriteData(io.Bytes())
}

//写入字节数组，包含了长度头
func (nb *nbuffer) WriteHeadData(bb []byte) {
	nb.writeLength(len(bb))
	nb.WriteData(bb)
}

//写入buffer中未读的字节，包含了长度头
func (nb *nbuffer) WriteHeadBuffer(io nbtcp.IoBuffer) {
	nb.WriteHeadData(io.Bytes())
}

func (nb *nbuffer) ReadHeadData() []byte {
	n := nb.readLength()
	bb := make([]byte, n, n)
	if n == 0 {
		return bb
	}
	err := binary.Read(nb.buf, DefaultEndian, bb)
	if err != nil {
		panic(err)
	}
	return bb
}

func (nb *nbuffer) ReadInt8() int8 {
	var x int8
	err := binary.Read(nb.buf, DefaultEndian, &x)
	if err != nil {
		panic(err)
	}
	return x
}
func (nb *nbuffer) ReadUint8() uint8 {
	var x uint8
	err := binary.Read(nb.buf, DefaultEndian, &x)
	if err != nil {
		panic(err)
	}
	return x
}
func (nb *nbuffer) ReadInt16() int16 {
	var x int16
	err := binary.Read(nb.buf, DefaultEndian, &x)
	if err != nil {
		panic(err)
	}
	return x
}
func (nb *nbuffer) ReadUint16() uint16 {
	var x uint16
	err := binary.Read(nb.buf, DefaultEndian, &x)
	if err != nil {
		panic(err)
	}
	return x
}
func (nb *nbuffer) ReadInt32() int32 {
	var x int32
	err := binary.Read(nb.buf, DefaultEndian, &x)
	if err != nil {
		panic(err)
	}
	return x
}
func (nb *nbuffer) ReadUint32() (n uint32) {
	err := binary.Read(nb.buf, DefaultEndian, &n)
	if err != nil {
		panic(err)
	}
	return
}
func (nb *nbuffer) ReadInt64() (n int64) {
	err := binary.Read(nb.buf, DefaultEndian, &n)
	if err != nil {
		panic(err)
	}
	return
}
func (nb *nbuffer) ReadUint64() (n uint64) {
	err := binary.Read(nb.buf, DefaultEndian, &n)
	if err != nil {
		panic(err)
	}
	return
}
func (nb *nbuffer) ReadStr() string {
	n := nb.readLength()
	if n == 0 {
		return ""
	}
	bb := make([]byte, n, n)
	err := binary.Read(nb.buf, DefaultEndian, bb)
	if err != nil {
		panic(err)
	}
	return string(bb)
}
func (nb *nbuffer) WriteStr(str string) {
	if str == "" {
		nb.writeLength(0)
		return
	}
	bb := []byte(str)
	nb.writeLength(len(bb))
	err := binary.Write(nb.buf, DefaultEndian, bb)
	if err != nil {
		panic(err)
	}
}
func (nb *nbuffer) ReadBool() bool {
	n := nb.ReadInt8()
	if n == 1 {
		return true
	} else {
		return false
	}
}
func (nb *nbuffer) ReadByte() byte {
	b, err := nb.buf.ReadByte()
	if err != nil {
		panic(err)
	}
	return b
}
func (nb *nbuffer) UnreadByte() {
	err := nb.buf.UnreadByte()
	if err != nil {
		panic(err)
	}
}
func (nb *nbuffer) ReadFloat32() float32 {
	x := nb.ReadUint32()
	return math.Float32frombits(x)
}
func (nb *nbuffer) ReadFloat64() float64 {
	x := nb.ReadUint64()
	return math.Float64frombits(x)
}

func (nb *nbuffer) WriteInt8(n int8) {
	err := binary.Write(nb.buf, DefaultEndian, n)
	if err != nil {
		panic(err)
	}
}
func (nb *nbuffer) WriteUint8(n uint8) {
	err := binary.Write(nb.buf, DefaultEndian, n)
	if err != nil {
		panic(err)
	}
}
func (nb *nbuffer) WriteInt16(n int16) {
	err := binary.Write(nb.buf, DefaultEndian, n)
	if err != nil {
		panic(err)
	}
}
func (nb *nbuffer) WriteUint16(n uint16) {
	err := binary.Write(nb.buf, DefaultEndian, n)
	if err != nil {
		panic(err)
	}
}
func (nb *nbuffer) WriteInt32(n int32) {
	err := binary.Write(nb.buf, DefaultEndian, n)
	if err != nil {
		panic(err)
	}
}
func (nb *nbuffer) WriteUint32(n uint32) {
	err := binary.Write(nb.buf, DefaultEndian, uint32(n))
	if err != nil {
		panic(err)
	}
}
func (nb *nbuffer) WriteInt64(n int64) {
	err := binary.Write(nb.buf, DefaultEndian, int64(n))
	if err != nil {
		panic(err)
	}
}
func (nb *nbuffer) WriteUint64(n uint64) {
	err := binary.Write(nb.buf, DefaultEndian, uint64(n))
	if err != nil {
		panic(err)
	}
}

func (nb *nbuffer) writeLength(n int) {
	if n >= 0x20000000 || n < 0 {
		panic(fmt.Errorf("writeLength, invalid len:%d", n))
	}
	if n < 0x80 {
		nb.WriteByte(byte(n + 0x80))
	} else if n < 0x4000 {
		nb.WriteInt16(int16(n + 0x4000))
	} else {
		nb.WriteInt32(int32(n + 0x20000000))
	}
}
func (nb *nbuffer) readLength() int {
	b := nb.ReadByte()
	n := int(b) & 0xff
	if n >= 0x80 {
		return n - 0x80
	}
	nb.UnreadByte()
	if n >= 0x40 {
		return int(nb.ReadUint16() - 0x4000)
	}
	if n >= 0x20 {
		return int(nb.ReadInt32() - 0x20000000)
	}
	panic(fmt.Errorf("readLength, invalid number:", n))
}

func (nb *nbuffer) WriteBool(n bool) {
	if n {
		nb.WriteInt8(1)
	} else {
		nb.WriteInt8(0)
	}
}

func (nb *nbuffer) WriteByte(n byte) {
	err := nb.buf.WriteByte(n)
	if err != nil {
		panic(err)
	}
}

func (nb *nbuffer) WriteFloat32(n float32) {
	x := math.Float32bits(n)
	nb.WriteUint32(x)
}

func (nb *nbuffer) WriteFloat64(n float64) {
	x := math.Float64bits(n)
	nb.WriteUint64(x)
}
func (nb *nbuffer) String() string {
	post := nb.GetPost()
	if post == 0 {
		post = timefix.MillisTime()
	}
	prct := nb.GetPrct()
	rcvt := nb.GetRcvt()
	if prct > 0 {
		if rcvt > 0 {
			//消息包总耗时
			tt3 := time.Duration(post - rcvt)
			//消息包等待耗时
			tt2 := time.Duration(prct - rcvt)
			//消息包处理耗时
			tt1 := time.Duration(post - prct)
			return fmt.Sprintf("req:%v ask:%v  wait:%v  proc:%v  cost:%v", nb.RcvPort(), nb.Port(), tt2, tt1, tt3)
		} else {
			//消息包等待耗时
			tt2 := time.Duration(prct - rcvt)
			//消息包处理耗时
			tt1 := time.Duration(post - prct)
			return fmt.Sprintf("req:%v ask:%v  wait:%v  proc:%v", nb.RcvPort(), nb.Port(), tt2, tt1)
		}
	} else if rcvt > 0 {
		//消息包总耗时
		tt3 := time.Duration(post - rcvt)
		return fmt.Sprintf("req:%v ask:%v  cost:%v", nb.RcvPort(), nb.Port(), tt3)
	} else {
		return fmt.Sprintf("req:%v ask:%v", nb.RcvPort(), nb.Port())
	}
}
