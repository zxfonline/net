// Copyright 2016 zxfonline@sina.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package packet

import (
	"io"
	"net"

	"golang.org/x/net/trace"
)

//连接提供器
type ConnectProducer interface {
	GetConnect() IoSession
}

//消息处理器
type MsgHandler interface {
	//消息处理方法(外部捕获异常，一般都是将连接关闭)
	Transmit(IoSession, IoBuffer)
}

//消息读取器
type MsgReader interface {
	//读取消息(调用者自行处理抛出的错误)
	ReadMsg() IoBuffer
}

//消息写入器
type MsgWriter interface {
	//写入后的消息是否加入字节缓存，如果是组播方式发送数据不能为true否则组播没完成可能被其他地方使用(调用者自行处理抛出的错误)
	WriteMsg(IoBuffer)
}

//消息读写器
type MsgReadWriter interface {
	MsgReader
	MsgWriter
}

//消息过滤器
type IoFilterChain interface {
	//连接开启，返回是否可以开启数据交换，false将关闭连接
	SessionOpening(IoSession) bool
	//连接开启成功
	SessionOpened(IoSession)
	//初始化加密
	InitEncrypt(int64, func(IoBuffer))
	//进行消息包解码、解压、解包等详细处理(调用者自行捕获错误)
	MessageReceived(IoSession, IoBuffer)
	//进行消息包编码、压缩、封装等详细处理(调用者自行捕获错误)
	MessageSend(IoSession, IoBuffer)
	//会话关闭
	SessionClosed(IoSession)
}

//socket连接封装器
type IoSession interface {
	//获取连接唯一ID
	GetCid() int64
	//返回本地网络地址
	LocalAddr() net.Addr
	//返回远程网络地址
	RemoteAddr() net.Addr
	//返回远程网络地址
	RemoteIP() string
	//关闭连接
	Close()
	//连接是否关闭
	Closed() bool
	//消息包最大字节,0表示无上限
	ReadBufMaxSize() uint
	//发送消息 如果参数非nbtcp.IoBuffer类型或写入时出错，内部会抛错，调用者自行处理异常
	Write(interface{})
	//初始化加密
	InitEncrypt(token int64)
	//设置连接数据源
	SetSource(interface{})
	//获取连接数据源
	GetSource() interface{}
}

//io消息封装器
type IoBuffer interface {
	io.ReaderFrom
	io.WriterTo
	io.Reader
	io.Writer
	//收到消息的时间
	GetRcvt() int64
	//收到消息的时间
	SetRcvt(int64)
	//开始处理时间
	GetPrct() int64
	//开始处理时间
	SetPrct(int64)
	//消息处理完成/发送时间
	GetPost() int64
	//消息处理完成/发送时间
	SetPost(int64)
	//消息归属的连接id
	ConnectID() int64
	//连接id
	SetConnectID(int64)
	//消息包发送类型
	Port() PackApi
	//消息包复用，一般处理完消息后直接复用该消息包，更改消息传输类型并填充数据回执给请求方
	SetPort(PackApi)
	//消息包接收类型，初次创建消息包传入的数据包类型
	RcvPort() PackApi
	SetRcvPort(PackApi)
	//消息唯一id
	Uuid() int64
	SetUuid(int64)
	//是否放入数组缓存
	Cached() bool
	//发送完缓存数据数据
	Cache(bool) IoBuffer
	//返回未读完数据
	Bytes() []byte
	//可读数据长度
	Len() int
	//字节缓存容量
	Cap() int
	//数据重置
	Reset() IoBuffer

	//写入字节数组，不包含了长度头
	WriteData([]byte)
	//写入buffer中未读的字节，不包含了长度头
	WriteBuffer(IoBuffer)
	//写入字节数组，包含了长度头
	WriteDataWithHead([]byte)
	//写入buffer中未读的字节，包含了长度头
	WriteBufferWithHead(IoBuffer)
	//读取字节数据，通过读取字节头判断字节长度
	ReadDataWithHead() []byte
	//读取所有剩余字节数据
	ReadData() []byte

	ReadLength() int
	ReadInt8() int8
	ReadUint8() uint8
	ReadInt16() int16
	ReadUint16() uint16
	ReadInt32() int32
	ReadUint32() uint32
	ReadInt64() int64
	ReadUint64() uint64
	ReadString() string
	ReadBool() bool
	ReadByte() byte
	ReadFloat32() float32
	ReadFloat64() float64

	WriteLength(int)
	WriteInt8(int8)
	WriteUint8(uint8)
	WriteInt16(int16)
	WriteUint16(uint16)
	WriteInt32(int32)
	WriteUint32(uint32)
	WriteInt64(int64)
	WriteUint64(uint64)
	WriteString(string)
	WriteBool(bool)
	WriteByte(byte)
	WriteFloat32(float32)
	WriteFloat64(float64)
	//将 v 编码写入到buffer中 head:是否有消息体长度 (eg:"encoding/gob","github.com/golang/protobuf/proto","http://msgpack.org/") 查看 zxfonline/net/packet.go
	WriteBinaryData(v interface{}, head bool)
	//读取一个data解码到v(a pointer interface{}) head:是否有消息体长度 (eg:"encoding/gob","github.com/golang/protobuf/proto","http://msgpack.org/")  查看 zxfonline/net/packet.go
	ReadBinaryData(v interface{}, head bool)

	String() string

	RegistTraceInfo(trace.Trace)
	TraceInfo() trace.Trace
	TracePrintf(format string, a ...interface{})
	TraceErrorf(format string, a ...interface{})
	TraceFinish()
}
