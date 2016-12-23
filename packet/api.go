// Copyright 2016 zxfonline@sina.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package packet

import (
	"strconv"
)

type MsgType int32

func (x MsgType) String() string {
	return EnumName(MsgType_name, int32(x))
}
func (x MsgType) Value() int32 {
	return int32(x)
}

// EnumName is a helper function to simplify printing enums by name.  Given an enum map and a value, it returns a useful string.
func EnumName(m map[int32]string, v int32) string {
	s, ok := m[v]
	if ok {
		return s
	}
	return strconv.Itoa(int(v))
}

var MsgType_name = map[int32]string{
	1: "REQ_TIME_PORT",
	2: "ACK_TIME_PORT",
	3: "ACCESS_RETURN_PORT",
}

//系统消息代理器类型常量
const (
	//时间请求端口
	REQ_TIME_PORT MsgType = 1
	//时间响应端口
	ACK_TIME_PORT MsgType = 2
	//消息访问返回端口
	ACCESS_RETURN_PORT MsgType = 3
)
