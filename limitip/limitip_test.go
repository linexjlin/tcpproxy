package limit

import (
	"testing"
)

func TestConnCheck(t *testing.T) {
	var con = newConn(1)
	if con.CheckAndAdd("127.0.0.1") {
		t.Log("OK")
	}
	if !con.CheckAndAdd("127.0.0.2") {
		t.Log("OK")
	}
}

func TestLIMITCheck(t *testing.T) {
	var l = NewLIMIT()
	if l.Check("user01", "127.0.0.1", 1) {
		t.Log("OK2")
	}
	if l.Check("user01", "127.0.0.1", 1) {
		t.Log("OK2")
	}
	if !l.Check("user01", "127.0.0.2", 1) {
		t.Log("OK2")
	}
}
