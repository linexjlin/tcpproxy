package tcplatency

import (
	"testing"
)

func TestDialLatency(t *testing.T) {
	if dur, err := dialLatency("www.baidu.com:80"); err != nil {
		t.Log(err)
		t.Fail()
	} else {
		t.Log("latency is:", dur)
	}
}

func TestLatency(t *testing.T) {
	t.Log(Latency("www.baidu.com:80"))
}
