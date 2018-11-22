package main

import (
	"flag"
	"log"

	tp "github.com/linexjlin/tcpproxy/tcpproxy"
)

var P *tp.Proxy

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)
	//conf := flag.String("f", "./proxy.conf", "/etc/proxy.conf")
	url := flag.String("u", "", "https://conf.site.com/conf.json")
	addBytesUrl := flag.String("a", "", "https://conf.site.com/conf.json")
	sendTraf := flag.Bool("t", false, "weather send host's traffic to remote")
	sendBytes := flag.Bool("b", false, "weather send bytes of host traffic")
	sendIP := flag.Bool("i", false, "weahter send IP of host")
	name := flag.String("n", "", "server name")
	flag.Parse()

	if *name == "" {
		log.Fatal("server name can't not be empty!")
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
