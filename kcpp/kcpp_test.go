package kcpp

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/linexjlin/simple-log"
)

func trans(p1, p2 io.ReadWriteCloser) (int64, int64) {
	var sync = make(chan int64, 2)
	var toP1Bytes, toP2Bytes int64
	go func() {
		n, err := io.Copy(p1, p2)
		if err != nil {
			log.Debug(err)
		}
		toP1Bytes = n
		sync <- n
	}()

	go func() {
		n, err := io.Copy(p2, p1)
		if err != nil {
			log.Debug(err)
		}
		toP2Bytes = n
		sync <- n
	}()

	<-sync
	select {
	case <-sync:
	case <-time.After(time.Second * 10):
	}
	p1.Close()
	p2.Close()
	return toP1Bytes, toP2Bytes
}

func forwarder(conn io.ReadWriteCloser, laddr, raddr net.Addr) {
	if backend, err := net.DialTimeout("tcp", "127.0.0.1:80", time.Millisecond*400); err != nil {
		log.Println(err)
	} else {
		trans(conn, backend)
	}
}

func TestListen(t *testing.T) {
	ListenKCP(":9000", forwarder)
}
