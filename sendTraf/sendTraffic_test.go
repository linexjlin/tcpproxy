package sendTraf

import (
	"testing"
)

func TestSendTraf(t *testing.T) {
	SendTraf("xxx.cdn.hp.avalon.pw", "xxx", "http://127.0.0.1:8035/addTraffic", "LA83", 12, 10)
}
