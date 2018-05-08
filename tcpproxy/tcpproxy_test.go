package tcpproxy

import (
	"log"
	"testing"
	"time"
)

var config Config

func TestInit(t *testing.T) {
	config.DefaultHTTPBackends = []string{"155.94.190.217:80", "155.94.190.218:80"}
	config.FailHTTPBackends = []string{"mirror.centos.org:80"}
	config.DefaultTCPBackends = []string{"pm.chinatcc.com:22"}
	config.Listen = []string{"0.0.0.0:58080", "0.0.0.0:58081"}
	config.Route = make(map[string][]string)
	config.Route["vbsxweim.cdn.avalon.pw"] = []string{"155.94.190.217:80", "155.94.190.218:80"}
	config.Route["6fto7ryo.cdn.avalon.pw"] = []string{"155.94.190.217:80", "155.94.190.218:80"}
	log.Println("Init complete!")
}

func TestStart(t *testing.T) {
	proxy := NewProxy()
	proxy.UpdateConfig(&config)
	proxy.Start()
	time.Sleep(time.Second * 500)
}
