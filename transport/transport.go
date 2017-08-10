// Copyright 2016 zxfonline@sina.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transport

import (
	"io"

	//	"github.com/zxfonline/fileutil"
	//	"fmt"
	//	"os"
	. "github.com/zxfonline/net/packet"
)

//默认消息包长度上限128k
const DefaultMaxMsg = 128 * 1024

//默认消息解码器注册机
var DefaultIoc = func(rw io.ReadWriter, session IoSession) MsgReadWriter {
	//输出到文件
	//	md := NewMsgRWDump(NewMsgRWIO(rw, DefaultMaxMsg), nil)
	//	if wc, err := fileutil.OpenFile(fmt.Sprintf("./log/dump/session_%d.txt", session.GetCid()), os.O_TRUNC|os.O_CREATE|os.O_WRONLY, fileutil.DefaultFileMode); err == nil {
	//		md.SetDump(wc)
	//	}
	//	return md
	md := NewMsgRWDump(NewMsgRWIO("io_service", rw, DefaultMaxMsg), func(IoBuffer) bool {
		return false
	})
	return md
}

//默认消息过滤器注册机
var DefaultIoFilter = func(session IoSession) IoFilterChain {
	return &DefaultIoFilterChain{}
}

type DefaultIoFilterChain struct {
}

//step1: IoFilterChain.SessionOpening() 黑名单等过滤检查连接是否允许被建立
func (r *DefaultIoFilterChain) SessionOpening(session IoSession) bool {
	return true
}

//step2: IoFilterChain.SessionOpened() 开启成功
func (r *DefaultIoFilterChain) SessionOpened(session IoSession) {

}

//初始化加密
func (r *DefaultIoFilterChain) InitEncrypt(token int64, callback func(IoBuffer)) {}

//step3: IoFilterChain.MessageReceived() 进行消息包解码、解压、解包等详细处理,返回成功才继续处理该条消息
func (r *DefaultIoFilterChain) MessageReceived(session IoSession, data IoBuffer) bool {
	return true
}

//step4: IoFilterChain.MessageSend() 进行消息包编码、压缩、封装等详细处理,返回成功才继续处理该条消息
func (r *DefaultIoFilterChain) MessageSend(session IoSession, data IoBuffer) bool {
	data.Cache(true)
	return true
}

//step5: IoFilterChain.SessionClosed() 连接关闭后续处理
func (r *DefaultIoFilterChain) SessionClosed(session IoSession) {
}

//默认消息处理器注册机
var DefaultMsgProcer = func(session IoSession) MsgHandler {
	return concurrentMsgHandler
}

//共享消息解析器
var concurrentMsgHandler MsgHandler = &DefaultMsgHandler{}

type DefaultMsgHandler struct {
}

//MsgHandler.Transmit() 写入什么消息，返回什么消息
func (r *DefaultMsgHandler) Transmit(session IoSession, data IoBuffer) {
	session.Write(data)
}
