package tcpproxy

import (
	"io"
	"log"
	"net"
	"regexp"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/linexjlin/tcpproxy/peektype"
	"github.com/linexjlin/tcpproxy/sendTraf"
)

type Config struct {
	Listen              []string
	Route               map[string][]string
	regRoute            map[*regexp.Regexp][]string
	DefaultHTTPBackends []string
	FailHTTPBackends    []string
	DefaultTCPBackends  []string
	DefaultSSHBackends  []string
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
	new.regRoute = make(map[*regexp.Regexp][]string)
	for rs, v := range new.Route {
		r := regexp.MustCompile(rs)
		new.regRoute[r] = v
	}
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
		}
		for r, h := range config.regRoute {
			if r.MatchString(host) {
				log.Println("RegRoute")
				return h
			}
		}

		log.Println("FailHTTPBackends")
		return config.FailHTTPBackends
	case "TCP":
		log.Println("DefaultTCPBackends")
		return config.DefaultTCPBackends
	case "SSH":
		log.Println("DefaultSSHBackends")
		return config.DefaultSSHBackends
	}
	log.Println("DefaultTCPBackends")
	return config.DefaultTCPBackends
}

func (p *Proxy) forwardHTTP(conn net.Conn, host string, dat []byte) {
	defer conn.Close()
	remotes := p.getRemotes("HTTP", host)
	if len(remotes) == 0 {
		return
	}
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

func (p *Proxy) forward(conn net.Conn, remotes []string, dat []byte) {
	defer conn.Close()
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
		_, err := io.Copy(client, conn)
		if err != nil {
			log.Println(err)
		}
		sync <- 1
	}()

	go func() {
		_, err := io.Copy(conn, client)
		if err != nil {
			log.Println(err)
		}
		sync <- 1
	}()

	<-sync
}

func (p *Proxy) forwardTCP(conn net.Conn, dat []byte) {
	remotes := p.getRemotes("TCP", "")
	if len(remotes) == 0 {
		conn.Close()
	} else {
		p.forward(conn, remotes, dat)
	}
}

func (p *Proxy) forwardSSH(conn net.Conn, dat []byte) {
	remotes := p.getRemotes("SSH", "")
	if len(remotes) == 0 {
		conn.Close()
	} else {
		p.forward(conn, remotes, dat)
	}
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
				dat, cType, res, err := peektype.PeekType(conn)
				if err != nil {
					log.Println(err)
					conn.Close()
				} else {
					switch cType {
					case peektype.HTTP:
						host := res.(string)
						log.Println("peeked host:", host)
						go p.forwardHTTP(conn, host, dat)
					case peektype.SSH:
						log.Println("SSH")
						go p.forwardSSH(conn, dat)
					case peektype.NORMALTCP:
						log.Println("TCP")
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
