package tcpproxy

import (
	"testing"
	"time"
)

var route *Route

func TestInit(t *testing.T) {
	route = NewRoute()
	var r1, r2, r3 Rule
	var b1, b2, b3 Backend
	r1.name = "LISTEN"
	r1.rtype = LISTEN
	b1.maxIP = 1
	b1.services = []string{"0.0.0.0:58081", "0.0.0.0:58080"}

	r2.name = "6fto7ryo.cdn.avalon.pw"
	r2.rtype = UHTTP
	b2.maxIP = 1
	b2.services = []string{"155.94.190.217:80", "155.94.190.218:80"}

	r3.name = "test.chinatcc.com"
	r3.rtype = UHTTP
	b3.maxIP = 1
	b3.services = []string{"www.baidu.com:80", "www.baidu.com:80"}
	route.rules[r1] = b1
	route.rules[r2] = b2
	route.rules[r3] = b3
}

func TestStart(t *testing.T) {
	proxy := NewProxy(false, false, false)
	proxy.Start(route)
	time.Sleep(time.Second * 500)
}
