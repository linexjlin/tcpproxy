package main

import (
	"flag"

	"fmt"
	"github.com/linexjlin/simple-log"
	tp "github.com/linexjlin/tcpproxy/tcpproxy"
	"time"
)

var P *tp.Proxy

func main() {
	url := flag.String("u", "", "https://conf.site.com/conf.json")
	addBytesUrl := flag.String("a", "", "https://conf.site.com/conf.json")
	sendTraf := flag.Bool("t", false, "weather send host's traffic to remote")
	sendBytes := flag.Bool("b", false, "weather send bytes of host traffic")
	sendIP := flag.Bool("i", false, "weahter send IP of host")
	name := flag.String("n", "", "server name")
	version := flag.Bool("version", false, "")
	fileLog := flag.String("log", "", "-log ./tcpp.log")
	wsLog := flag.String("wslog", "", "-wslog :8044")
	autoUpdate := flag.String("update", "", "http://up.xxx.com/tcpp")
	debug := flag.Bool("debug", false, "debug")
	flag.Parse()
	time.Sleep(time.Second * 1)

	if *version == true {
		data, _ := Asset(".git/logs/HEAD")
		fmt.Println(string(data))
		return
	}
	if *name == "" {
		log.Fatal("server name can't not be empty!")
	}
	log.DebugEanble(*debug)

	if *fileLog != "" {
		log.LogToFile(*fileLog)
	}
	if *wsLog != "" {
		log.LogToWs(*wsLog, "/")

	}
	if *autoUpdate != "" {
		go updater(*autoUpdate)
	}
	P = tp.NewProxy(*sendTraf, *sendBytes, *sendIP, *addBytesUrl, *name)

	if config, err := getConfig(*url, *name, ""); err != nil {
		log.Fatal(err)
	} else {
		r := config2route(&config)

		//log.Println("get route", r)
		P.SetRoute(r)
		P.Start()
		if (*url) != "" {
			autoUpdateConfig(*url, *name)
		}
	}
}
