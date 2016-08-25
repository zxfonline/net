// Copyright 2016 zxfonline@sina.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package nbtcp

//服务器基础属性参数
type ServerConfig struct {
	//服务器消息执行并发线程数
	MultipleSize uint `json:"MultipleSize"`
	//服务器消息线程消息管道缓冲上限
	MultipleChanSize uint `json:"MultipleChanSize"`
	//连接管道读取数据缓冲大小
	ChanReadSize uint `json:"ChanReadSize"`
	//连接管道发送数据缓冲大小
	ChanSendSize uint `json:"ChanSendSize"`
	//消息包最大字节数,0表示无上限
	ReadBufMaxSize uint `json:"ReadBufMaxSize"`
	//连接空闲超时时间 秒 0 表示不做超时断开
	DeadlineSecond uint `json:"DeadlineSecond"`
}
