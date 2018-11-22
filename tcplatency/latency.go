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

type Latency struct {
	host map[string]time.Duration
}

func NewLatency() *Latency {
	var l Latency
	l.host = make(map[string]time.Duration)
	return &l
}

func (l *Latency) Order(hosts []string) {
	for _, addr := range hosts {
		if t, ok := l.host[addr]; !ok {
			latency := addrLatency(addr)
			log.Println(addr, latency)
			l.host[addr] = latency
		} else {
			log.Println(addr, "pass", t)
		}
	}

	for i := 0; i < len(hosts)-1; i++ {
		var min = i
		for j := i; j < len(hosts); j++ {
			if l.host[hosts[j]] < l.host[hosts[min]] {
				min = j
			}
		}
		//log.Println("swap")
		hosts[i], hosts[min] = hosts[min], hosts[i]
	}
}

func (l *Latency) updateLatency() {
	for addr, _ := range l.host {
		latency := addrLatency(addr)
		log.Println(addr, latency)
		l.host[addr] = latency
	}
}

func (l *Latency) AutoUpdateLatency() {
	log.Println("AutoUpdateLatency start")
	for {
		time.Sleep(time.Minute * 15)
		l.updateLatency()
	}
}

func addrLatency(addr string) (latency time.Duration) {
	var begin = time.Now()
	var pass = begin
	for i := 0; i < testCnt; i++ {
		if d, e := dialLatency(addr); e != nil {
			pass = pass.Add(DTimeOut)
		} else {
			//log.Println("latency to", addr, d)
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
