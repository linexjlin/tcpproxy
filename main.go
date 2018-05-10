package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/linexjlin/tcpproxy/tcpproxy"
)

var P = tcpproxy.NewProxy()

func autoUpdateConfig(url string, done chan int) {
	var config tcpproxy.Config
	for {
		if rsp, err := http.Get(url); err != nil {
			log.Println(err)
		} else {
			if err = json.NewDecoder(rsp.Body).Decode(&config); err != nil {
				log.Println(err)
			} else {
				log.Println("config", config)
				rsp.Body.Close()
				P.UpdateConfig(&config)
				done <- 1
			}
		}
		time.Sleep(time.Minute * 10)
	}
}

func main() {
	conf := flag.String("f", "./proxy.conf", "/etc/proxy.conf")
	url := flag.String("u", "", "https://conf.site.com/conf.json")
	sendTraf := flag.Bool("t", false, "true")
	sendBytes := flag.Bool("b", false, "true")
	flag.Parse()
	P.SendTraf = (*sendTraf)
	P.SendByes = (*sendBytes)
	log.Println(P.SendTraf, P.SendByes)
	if (*url) != "" {
		var done = make(chan int, 10)
		go autoUpdateConfig((*url), done)
		<-done
		go func() { <-done }()
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
