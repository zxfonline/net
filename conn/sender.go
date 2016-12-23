// Copyright 2016 zxfonline@sina.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package conn

import (
	"github.com/zxfonline/chanutil"
	"github.com/zxfonline/gerror"
)

//获取连接发送器
func newsender(c *Connect) *sender {
	s := &sender{
		writeChan: make(chan interface{}, c.ChanSendSize),
		stopD:     chanutil.NewDoneChan(),
		c:         c,
	}
	return s
}

type sender struct {
	//消息发送缓冲管道
	writeChan chan interface{}
	stopD     chanutil.DoneChan
	c         *Connect
}

func (s *sender) Start() {
	defer close(s.writeChan)
	for q := false; !q; {
		select {
		case <-s.stopD:
			q = true
		case out := <-s.writeChan:
			s.c.transfer(out)
		}
	}
}
func (s *sender) Close() {
	s.stopD.SetDone()
}

func (s *sender) Write(out interface{}) (err error) {
	defer gerror.PanicToErr(&err)
	s.writeChan <- out
	return
}
