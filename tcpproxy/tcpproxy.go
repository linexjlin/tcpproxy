package tcpproxy

import (
	"io"
	"log"
	"net"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/linexjlin/peektype"
	limit "github.com/linexjlin/tcpproxy/limitip"
	"github.com/linexjlin/tcpproxy/sendTraf"
)

const (
	UHTTP = iota
	UHTTPS
	NHTTP
	NHTTPS
	FHTTP
	FHTTPS
	SSH
	LISTEN
	UNKNOWN
)

type Rule struct {
	rtype int
	name  string
}

type Backend struct {
	services []string
	maxIP    int
	policy   string
}

type Route struct {
	rules map[Rule]Backend
}

func (r *Route) Add(rtype int, name string, maxIP int, policy string, services []string) {
	var rule Rule
	rule.name = name
	rule.rtype = rtype

	var backend Backend
	backend.maxIP = maxIP
	backend.services = services
	if _, ok := r.rules[rule]; !ok {
		r.rules[rule] = backend
	}
}

func NewRoute() *Route {
	var r Route
	r.rules = make(map[Rule]Backend)
	return &r
}

/*
type Config struct {
	Listen               []string
	Route                map[string][]string
	regRoute             map[*regexp.Regexp][]string
	DefaultHTTPBackends  []string
	DefaultHTTPSBackends []string
	FailHTTPBackends     []string
	DefaultTCPBackends   []string
	DefaultSSHBackends   []string
	AddByteUrl           string
}*/

type Taf struct {
	in  uint64
	out uint64
	ip  string
}

type Proxy struct {
	route                      *Route
	sendTraf, sendByes, sendIP bool
	ut                         map[string]*Taf
	listeners                  map[string]net.Listener
	AddByteUrl                 string
}

func NewProxy(sendTraf, sendByes, sendIP bool) *Proxy {
	p := Proxy{}
	p.sendTraf = sendTraf
	p.sendByes = sendByes
	p.sendIP = sendIP
	p.ut = make(map[string]*Taf)
	p.listeners = make(map[string]net.Listener)
	return &p
}

func (p *Proxy) SetRoute(route *Route) {
	p.route = route
	p.checkListenAndProxy()
}

func (p *Proxy) checkListenAndProxy() {
	if b, ok := p.route.rules[Rule{LISTEN, "LISTEN"}]; ok {
		if len(b.services) > 0 {
			for _, addr := range b.services {
				if _, ok := p.listeners[addr]; !ok {
					go p.listenAndProxy(addr)
				}
			}
		}
	}
}

func (p *Proxy) getRemotes(rType, host string) []string {
	switch rType {
	case "HTTP":
		if b, ok := p.route.rules[Rule{UHTTP, host}]; ok {
			if len(b.services) > 0 {
				log.Println("User HTTP")
				return b.services
			} else {
				log.Println("System HTTP Backends")
				return p.route.rules[Rule{NHTTP, host}].services
			}
		}
		log.Println("Unknown HTTP Backends", host)
		return p.route.rules[Rule{FHTTP, host}].services
	case "HTTPS":
		if b, ok := p.route.rules[Rule{UHTTPS, host}]; ok {
			if len(b.services) > 0 {
				log.Println("User HTTPS")
				return b.services
			} else {
				log.Println("System HTTPS Backends")
				return p.route.rules[Rule{NHTTPS, host}].services
			}
		}
		log.Println("Unknown HTTPS Backends", host)
		return p.route.rules[Rule{FHTTPS, host}].services
	case "SSH":
		host = "SSH"
		if b, ok := p.route.rules[Rule{SSH, host}]; ok {
			if len(b.services) > 0 {
				log.Println("UserRoute")
				return b.services
			}
		}
		host = "UNKNOWN"
		return p.route.rules[Rule{UNKNOWN, host}].services
	default:
		host = "UNKNOWN"
		return p.route.rules[Rule{UNKNOWN, host}].services
	}
}

func (p *Proxy) forward(conn net.Conn, remotes []string, dat []byte) int64 {
	var client net.Conn
	var err error
	for _, remote := range remotes {
		if client, err = net.DialTimeout("tcp", remote, time.Millisecond*400); err != nil {
			log.Println(remote, err)
			continue
		} else {
			//log.Println(conn.RemoteAddr(), "->", client.RemoteAddr())
			var sync = make(chan int64, 2)

			go func() {
				client.Write(dat)
				n, _ := io.Copy(client, conn)
				sync <- n
			}()

			go func() {
				n, _ := io.Copy(conn, client)
				sync <- n
			}()

			bytes := <-sync
			select {
			case bytes = <-sync:
			case <-time.After(time.Second * 20):
			}
			conn.Close()
			client.Close()
			return bytes
		}
		log.Println("All backend servers are die!")
		conn.Close()
	}
	return 0
}

func (p *Proxy) Start(route *Route) {
	p.SetRoute(route)
	if p.sendTraf {
		go p.autoSentTraf(time.Minute * 2)
	}
}

var LIM = limit.NewLIMIT()

func (p *Proxy) listenAndProxy(listenAddr string) {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Println("Failed to setup listener:", err)
		return
	} else {
		log.Println("Listen on", listenAddr)
	}

	for {
		if conn, err := listener.Accept(); err != nil {
			log.Println(err)
		} else {
			//log.Println("accept connectino from:", conn.RemoteAddr())
			ip := strings.Split(conn.RemoteAddr().String(), ":")[0]
			go func() {
				var buf = make([]byte, 512)
				if n, e := conn.Read(buf); e != nil {
					log.Println(e)
				} else {
					buf = buf[:n]
					peek := peektype.NewPeek()
					peek.SetBuf(buf)
					t := peek.Parse()

					var remotes []string
					var hostname = peek.Hostname
					if hostname != "" && !LIM.Check(hostname, ip) {
						log.Println("Max IP reach:", hostname, ip)
						conn.Close()
						return
					}
					switch t {
					case peektype.SSH:
						remotes = p.getRemotes("SSH", "")
					case peektype.HTTP:
						remotes = p.getRemotes("HTTP", peek.Hostname)
						log.Println("peekhost:", hostname)
					case peektype.HTTPS:
						remotes = p.getRemotes("HTTPS", peek.Hostname)
						log.Println("peekhost:", hostname)
					case peektype.UNKNOWN:
						remotes = p.getRemotes("TCP", peek.Hostname)
					}

					if len(remotes) == 0 {
						log.Println("Unable to find remote hosts")
						conn.Close()
					} else {
						n := p.forward(conn, remotes, buf[:n])
						user := hostname
						if _, ok := p.ut[user]; !ok {
							p.ut[user] = &Taf{}
							p.ut[user].ip = ip
						}
						log.Println(hostname, "(", ip, ")", "->", remotes[0], " traffic:", n)
						p.ut[user].in += uint64(n)
						p.ut[user].out += uint64(n)
					}
				}
			}()
		}
	}
}

func (p *Proxy) autoSentTraf(interval time.Duration) {
	var from = time.Now()
	for {
		time.Sleep(interval)
		for u, t := range p.ut {
			if t.out == 0 {
				continue
			} else {
				if !p.sendIP {
					t.ip = ""
				}
				if p.sendByes {
					sendTraf.SendTraf(u, t.ip, p.AddByteUrl, uint64(t.in), uint64(t.out))
				} else {
					sendTraf.SendTraf(u, t.ip, p.AddByteUrl, 0, 0)
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
