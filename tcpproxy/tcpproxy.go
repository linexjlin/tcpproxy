package tcpproxy

import (
	"io"
	"log"
	"net"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/linexjlin/tcpproxy/peekhost"
	"github.com/linexjlin/tcpproxy/sendTraf"
)

type Config struct {
	Listen              []string
	Route               map[string][]string
	DefaultHTTPBackends []string
	FailHTTPBackends    []string
	DefaultTCPBackends  []string
	AddByteUrl          string
}

type Taf struct {
	in  uint64
	out uint64
	ip  string
}

type Proxy struct {
	cfg                        *Config
	SendTraf, SendByes, SendIP bool
	ut                         map[string]*Taf
}

func NewProxy() *Proxy {
	p := Proxy{}
	p.ut = make(map[string]*Taf)
	return &p
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

func (p *Proxy) forwardHTTP(conn net.Conn, host string, dat []byte) {
	defer conn.Close()
	remotes := p.getRemotes("HTTP", host)
	var client net.Conn
	var err error
	for i, remote := range remotes {
		client, err = net.DialTimeout("tcp", remote, time.Millisecond*400)
		if err == nil {
			defer client.Close()
			break
		} else {
			log.Println(err)
			if i+1 == len(remotes) {
				log.Println("All backend server die! Unfortunate:", host)
				time.Sleep(time.Second * 2)
				return
			} else {
				continue
			}
		}
	}

	log.Println(conn.RemoteAddr(), "->", host, "->", client.RemoteAddr())
	user := host
	userIP := strings.Split(conn.RemoteAddr().String(), ":")[0]
	if _, ok := p.ut[user]; !ok {
		p.ut[user] = &Taf{}
		p.ut[user].ip = userIP
	}

	var sync = make(chan int, 2)
	go func() {
		client.Write(dat)
		bytes, _ := io.Copy(client, conn)
		p.ut[user].in += uint64(bytes)
		sync <- 1
	}()

	go func() {
		bytes, _ := io.Copy(conn, client)
		p.ut[user].out += uint64(bytes)
		sync <- 1
	}()

	<-sync
}

func (p *Proxy) forwardTCP(conn net.Conn, dat []byte) {
	defer conn.Close()
	remotes := p.getRemotes("TCP", "")
	var client net.Conn
	var err error
	for i, remote := range remotes {
		client, err = net.DialTimeout("tcp", remote, time.Millisecond*400)
		if err == nil {
			defer client.Close()
			break
		} else {
			log.Println(err)
			if i+1 == len(remotes) {
				log.Println("all backend server die")
				time.Sleep(time.Second * 2)
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

	<-sync
}

func (p *Proxy) Start() {
	if p.SendTraf {
		go p.AutoSentTraf(time.Minute * 2)
	}
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
		if conn, err := listener.Accept(); err != nil {
			log.Println(err)
		} else {
			log.Println("accept connectino from:", conn.RemoteAddr())
			go func() {
				dat, host, err := peekhost.PeekHost(conn)
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
			}()
		}
	}
}

func (p *Proxy) AutoSentTraf(interval time.Duration) {
	var from = time.Now()
	for {
		time.Sleep(interval)
		for u, t := range p.ut {
			if t.out == 0 {
				continue
			} else {
				if !p.SendIP {
					t.ip = ""
				}
				if p.SendByes {
					sendTraf.SendTraf(u, t.ip, p.cfg.AddByteUrl, uint64(t.in), uint64(t.out))
				} else {
					sendTraf.SendTraf(u, t.ip, p.cfg.AddByteUrl, 0, 0)
				}
				log.Println(t.ip, u, humanize.Bytes(uint64(float64(t.out)/time.Now().Sub(from).Seconds())), "/s↓",
					humanize.Bytes(uint64(float64(t.in)/time.Now().Sub(from).Seconds())), "/s↑")
				t.out = 0
				t.in = 0
			}
		}
		from = time.Now()
	}
}
