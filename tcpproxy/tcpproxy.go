package tcpproxy

import (
	"io"
	"log"
	"net"
	"time"

	"github.com/linexjlin/tcpproxy/peekhost"
)

type Config struct {
	Listen              []string
	Route               map[string][]string
	DefaultHTTPBackends []string
	FailHTTPBackends    []string
	DefaultTCPBackends  []string
	AddByteUrl          string
}

type Proxy struct {
	cfg *Config
}

func (p *Proxy) UpdateConfig(new *Config) {
	p.cfg = new
}

func (p *Proxy) GetConfig() *Config {
	return p.cfg
}

func (p *Proxy) getRemotes(rType, host string) []string {
	config := p.GetConfig()
	switch rType {
	case "HTTP":
		if r, ok := config.Route[host]; ok {
			if len(r) == 0 {
				log.Println("DefaultHTTPBackends")
				return config.DefaultHTTPBackends
			} else {
				log.Println("UserRoute")
				return r
			}
		} else {
			log.Println("FailHTTPBackends")
			return config.FailHTTPBackends
		}
	case "TCP":
		log.Println("DefaultTCPBackends")
		return config.DefaultTCPBackends
	}
	log.Println("DefaultTCPBackends")
	return config.DefaultTCPBackends
}

func NewProxy() *Proxy {
	return &Proxy{}
}

func (p *Proxy) forwardHTTP(conn net.Conn, host string, dat []byte) {
	remotes := p.getRemotes("HTTP", host)
	var client net.Conn
	var err error
	for i, remote := range remotes {
		client, err = net.DialTimeout("tcp", remote, time.Millisecond*400)
		if err == nil {
			break
		} else {
			if i+1 == len(remotes) {
				log.Println("All backend server die! Unfortunate:", host)
				time.Sleep(time.Second * 2)
				conn.Close()
				return
			} else {
				continue
			}

		}
	}

	var sync = make(chan int, 2)
	log.Println(conn.RemoteAddr(), "->", host, "->", client.RemoteAddr())
	go func() {
		client.Write(dat)
		io.Copy(client, conn)
		sync <- 1
	}()

	go func() {
		io.Copy(conn, client)
		sync <- 1
	}()

	go func() {
		<-sync
		client.Close()
		conn.Close()
	}()
}

func (p *Proxy) forwardTCP(conn net.Conn, dat []byte) {
	remotes := p.getRemotes("TCP", "")
	var client net.Conn
	var err error
	for i, remote := range remotes {
		client, err = net.DialTimeout("tcp", remote, time.Millisecond*400)
		if err == nil {
			break
		} else {
			log.Println(err)
			if i+1 == len(remotes) {
				log.Println("all backend server die")
				time.Sleep(time.Second * 2)
				conn.Close()
				return
			} else {
				continue
			}

		}
	}

	log.Println(conn.RemoteAddr(), "->", client.RemoteAddr())
	var sync = make(chan int, 2)

	go func() {
		client.Write(dat)
		io.Copy(client, conn)
		sync <- 1
	}()

	go func() {
		io.Copy(conn, client)
		sync <- 1
	}()

	go func() {
		<-sync
		client.Close()
		conn.Close()
	}()
}

func (p *Proxy) Start() {
	p.listenAndProxyAll()
}

func (p *Proxy) listenAndProxyAll() {
	config := p.GetConfig()
	for _, addr := range config.Listen {
		go p.listenAndProxy(addr)
	}
}

func (p *Proxy) listenAndProxy(listenAddr string) {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("Failed to setup listener: %v", err)
	} else {
		log.Println("Listen on", listenAddr)
	}

	for {
		conn, err := listener.Accept()
		dat, host, err := peekhost.PeekHost(conn)
		log.Println("accept connectino from:", conn.RemoteAddr())
		log.Println("peeked host:", host)
		if err != nil {
			log.Println("A TCP Connection", err)
			go p.forwardTCP(conn, dat)
		} else {
			if host != "" {
				go p.forwardHTTP(conn, host, dat)
			} else {
				go p.forwardTCP(conn, dat)
			}
		}

	}
}
