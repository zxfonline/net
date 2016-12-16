// Copyright 2016 zxfonline@sina.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package transport

import (
	"github.com/zxfonline/golog"
	"github.com/zxfonline/net/nbtcp"
	"github.com/zxfonline/timefix"
)

//服务器时间响应接口
func NewTimeAckHandler() *SafeTransmitHandler {
	return NewSafeTransmitHandler(golog.New("TimeAckHandler"), func(connect nbtcp.IoSession, in nbtcp.IoBuffer, logger *golog.Logger) {
		//获取到远程服务器时间 毫秒
		remote_time := in.ReadInt64()
		timefix.ResetTime(remote_time)
	})
}

//服务器时间请求接口
func NewTimeReqHandler() *SafeTransmitHandler {
	return NewSafeTransmitHandler(golog.New("TimeReqHandler"), func(connect nbtcp.IoSession, in nbtcp.IoBuffer, logger *golog.Logger) {
		//发送本地纠正的时间 毫秒
		in.Reset()
		in.SetPort(ACK_TIME_PORT)
		in.WriteInt64(timefix.MillisUTCTime())
		connect.Write(in)
	})
}
