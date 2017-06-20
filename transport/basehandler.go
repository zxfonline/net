// Copyright 2016 zxfonline@sina.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transport

import (
	"github.com/zxfonline/golog"
	. "github.com/zxfonline/net/packet"
	"github.com/zxfonline/timefix"
)

//服务器时间响应接口
func NewTimeAckHandler(msgtype PackApi) *SafeTransmitHandler {
	return NewSafeTransmitHandler(golog.New(msgtype.String()), func(connect IoSession, in IoBuffer, logger *golog.Logger) {
		//获取到远程服务器时间 纳秒
		remote_time := in.ReadInt64()
		timefix.ResetTime(remote_time)
	})
}

//服务器时间请求接口
func NewTimeReqHandler(msgtype PackApi) *SafeTransmitHandler {
	return NewSafeTransmitHandler(golog.New(msgtype.String()), func(connect IoSession, in IoBuffer, logger *golog.Logger) {
		//发送本地纠正的时间 毫秒
		in.Reset()
		in.SetPort(ACK_TIME_PORT)
		in.WriteInt64(timefix.NanosTime())
		connect.Write(in)
	})
}
