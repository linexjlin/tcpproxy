package tcplatency

import (
	"testing"
)

var Lat = NewLatency()

func TestDialLatency(t *testing.T) {
	if dur, err := dialLatency("www.baidu.com:80"); err != nil {
		t.Log(err)
		t.Fail()
	} else {
		t.Log("latency is:", dur)
	}
}

/*
func TestLatency(t *testing.T) {
	t.Log(Latency("www.baidu.com:80"))
}*/

func TestOrder(t *testing.T) {
	origin := []string{"www.usa.com:80", "t.cn:80", "www.baidu.com:443", "jandan.net:80"}
	Lat.Order(origin)
}

/*
func TestOrderHostByBackup(t *testing.T) {
	origin := []string{"www.usa.com:80", "www.baidu.com:80"}
	ordered := []string{"www.usa.com:80", "www.baidu.com:80"}
	OrderHostByBackup(ordered)
	if origin[0] == ordered[0] {
		t.Log(ordered)
		t.Fail()
	} else {
		t.Log(ordered)
	}
}*/
