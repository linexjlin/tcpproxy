package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/linexjlin/tcpproxy/tcplatency"
	"github.com/linexjlin/tcpproxy/tcpproxy"
)

var P = tcpproxy.NewProxy()

func autoUpdateConfig(url string, done chan int) {
	var config tcpproxy.Config
	var loadCnt = 0
	for {
		if rsp, err := http.Get(url); err != nil {
			log.Println(err)
		} else {
			if err = json.NewDecoder(rsp.Body).Decode(&config); err != nil {
				log.Println(err)
			} else {
				log.Println("config", config)
				rsp.Body.Close()
				if loadCnt == 0 {
					done <- 1
				} else {
					optimizeBackend(&config)
				}
				loadCnt++
				P.UpdateConfig(&config)
			}
		}
		time.Sleep(time.Minute * 10)
	}
}

func optimizeBackend(c *tcpproxy.Config) {
	tcplatency.OrderHostByBackup(c.DefaultHTTPBackends)
	tcplatency.OrderHostByBackup(c.DefaultTCPBackends)
	tcplatency.OrderHostByBackup(c.FailHTTPBackends)
}

func main() {
	conf := flag.String("f", "./proxy.conf", "/etc/proxy.conf")
	url := flag.String("u", "", "https://conf.site.com/conf.json")
	sendTraf := flag.Bool("t", false, "weather send host's traffic to remote")
	sendBytes := flag.Bool("b", false, "weather send bytes of host traffic")
	sendIP := flag.Bool("i", false, "weahter send IP of host")
	flag.Parse()
	P.SendTraf = (*sendTraf)
	P.SendByes = (*sendBytes)
	P.SendIP = (*sendIP)
	log.Println(P.SendTraf, P.SendByes)
	if (*url) != "" {
		var done = make(chan int, 10)
		go autoUpdateConfig((*url), done)
		<-done
		goto STARTPROXY
	}

	if (*conf) != "" {
		var config tcpproxy.Config
		if f, err := os.Open(*conf); err != nil {
			log.Println(err)
			return
		} else {
			if err = json.NewDecoder(f).Decode(&config); err != nil {
				log.Println(err)
				return
			} else {
				P.UpdateConfig(&config)
			}
		}
	}
STARTPROXY:
	P.Start()

	select {}
}
