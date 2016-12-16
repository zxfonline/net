package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zxfonline/timer"

	"github.com/zxfonline/net/transport"
)

func main1() {
	producer := transport.NewTimerTcpConnectProducer("test", nil, "127.0.0.1:8888")
	producer.TimeFixStart(timer.GTimer(), 3*time.Second)
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	<-ch
}
