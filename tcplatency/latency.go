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

func OrderHostByBackup(hosts []string) {
	if len(hosts) < 2 {
		return
	} else {
		firstLatency := Latency(hosts[0])
		log.Println("Latency of", hosts[0], firstLatency)
		for i, host := range hosts {
			if i == 0 {
				continue
			} else {
				l := Latency(host)
				log.Println("Latency of", host, l)
				if time.Duration(firstLatency.Nanoseconds()-l.Nanoseconds()) > time.Millisecond*120 {
					log.Println("switch", hosts[0], host)
					firstLatency = l
					th := hosts[0]
					hosts[0] = host
					hosts[i] = th
				}
			}
		}
	}
}

func OrderByLatency(hosts []string) {

}

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
