// Copyright 2016 zxfonline@sina.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package nbtcp

import (
	"fmt"

	"github.com/zxfonline/chanutil"
)

//获取连接发送器
func NewSender(c *Connect) *Sender {
	s := &Sender{
		writeChan: make(chan interface{}, c.ChanSendSize),
		stopD:     chanutil.NewDoneChan(),
		c:         c,
	}
	return s
}

type Sender struct {
	//消息发送缓冲管道
	writeChan chan interface{}
	stopD     chanutil.DoneChan
	c         *Connect
}

func (s *Sender) Start() {
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
func (s *Sender) Close() {
	s.stopD.SetDone()
}

func (s *Sender) Write(out interface{}) (err error) {
	defer func() {
		if e := recover(); e != nil {
			switch e.(type) {
			case error:
				err = e.(error)
			default:
				err = fmt.Errorf("%v", e)
			}
		}
	}()
	s.writeChan <- out
	return
}
