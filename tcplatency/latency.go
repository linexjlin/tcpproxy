package tcplatency

import (
	"log"
	"net"
	"time"
)

const (
	DTimeOut     = time.Second * 2
	testCnt      = 10
	testInterval = time.Millisecond * 600
)

func Latency(addr string) (latency time.Duration) {
	var begin = time.Now()
	var pass = begin
	for i := 0; i < testCnt; i++ {
		if d, e := dialLatency(addr); e != nil {
			pass = pass.Add(DTimeOut)
		} else {
			log.Println("latency to", addr, d)
			pass = pass.Add(d)
		}
		time.Sleep(testInterval)
	}
	return time.Duration(pass.Sub(begin).Nanoseconds() / testCnt)
}

func dialLatency(addr string) (dur time.Duration, err error) {
	var begin = time.Now()
	if conn, err := net.DialTimeout("tcp", addr, DTimeOut); err != nil {
		return DTimeOut, err
	} else {
		defer conn.Close()
		return time.Now().Sub(begin), nil
	}
}
