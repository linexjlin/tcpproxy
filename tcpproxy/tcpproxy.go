package tcpproxy

import (
	"io"
	"net"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/linexjlin/peektype"
	"github.com/linexjlin/simple-log"
	"github.com/linexjlin/tcpproxy/kcpp"
	limit "github.com/linexjlin/tcpproxy/limitip"
	"github.com/linexjlin/tcpproxy/sendTraf"
	tl "github.com/linexjlin/tcpproxy/tcplatency"
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

func NewRoute() *Route {
	var r Route
	r.rules = make(map[Rule]Backend)
	r.latency = tl.NewLatency()
	go r.latency.AutoUpdateLatency()
	return &r
}

type Route struct {
	rules   map[Rule]Backend
	latency *tl.Latency
}

func (r *Route) Add(rtype int, name string, maxIP int, policy string, services []string) {
	var rule Rule
	rule.name = name
	rule.rtype = rtype

	var backend Backend
	backend.maxIP = maxIP
	backend.services = services
	backend.policy = policy
	if _, ok := r.rules[rule]; !ok {
		r.rules[rule] = backend
	}
}

func (r *Route) OptimizeBackend() {
	log.Info("Optimize backend start")
	for rule, backends := range r.rules {
		if rule.rtype != LISTEN && backends.policy == "latency" {
			log.Println("order", backends.services, "by", backends.policy)
			r.latency.Order(backends.services)
			log.Println("after order", backends.services)
		}
	}
}

func (r *Route) PrintRules() {
	for k, v := range r.rules {
		log.Println(k, v)
	}
}

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
	addByteUrl                 string
	name                       string
}

func NewProxy(sendTraf, sendByes, sendIP bool, url, name string) *Proxy {
	p := Proxy{}
	p.sendTraf = sendTraf
	p.sendByes = sendByes
	p.sendIP = sendIP
	p.addByteUrl = url
	p.name = name
	p.ut = make(map[string]*Taf)
	p.listeners = make(map[string]net.Listener)
	return &p
}

func (p *Proxy) SetRoute(route *Route) {
	p.route = route
	//p.route.PrintRules()
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

func (p *Proxy) getRemotes(rType, host, ip string) []string {
	switch rType {
	case "HTTP":
		if b, ok := p.route.rules[Rule{UHTTP, host}]; ok {
			if LIM.Check(host, ip, b.maxIP) {
				if len(b.services) > 0 {
					log.Println("User HTTP")
					return b.services
				} else {
					log.Println("System HTTP Backends")
					return p.route.rules[Rule{NHTTP, ""}].services
				}
			} else {
				log.Warning("Max IP reached", host, ip, b.maxIP)
			}

		}
		log.Println("Unknown HTTP Backends", host)
		return p.route.rules[Rule{FHTTP, ""}].services
	case "HTTPS":
		if b, ok := p.route.rules[Rule{UHTTPS, host}]; ok {
			if LIM.Check(host, ip, b.maxIP) {
				if len(b.services) > 0 {
					log.Println("User HTTPS")
					return b.services
				} else {
					log.Println("System HTTPS Backends")
					return p.route.rules[Rule{NHTTPS, ""}].services
				}
			} else {
				log.Warning("Max IP reached", host, ip, b.maxIP)
			}
		}
		log.Println("Unknown HTTPS Backends", host)
		return p.route.rules[Rule{FHTTPS, ""}].services
	case "SSH":
		host = "SSH"
		if b, ok := p.route.rules[Rule{SSH, ""}]; ok {
			if LIM.Check(host, ip, b.maxIP) {
				if len(b.services) > 0 {
					log.Println("UserRoute")
					return b.services
				}
			} else {
				log.Warning("Max IP reached", host, ip, b.maxIP)
			}

		}

		host = "UNKNOWN"
		return p.route.rules[Rule{UNKNOWN, ""}].services
	default:
		host = "UNKNOWN"
		return p.route.rules[Rule{UNKNOWN, ""}].services
	}
}

//io.CopyBuffer
func trans3(p1, p2 io.ReadWriteCloser) (int64, int64) {
	var sync = make(chan int, 2)
	var toP1Bytes, toP2Bytes int64
	var err error
	go func() {
		buf := make([]byte, 65535)
		toP1Bytes, err = io.CopyBuffer(p1, p2, buf)
		if err != nil {
			log.Debug(err)
		}
		sync <- 1
	}()

	go func() {
		buf := make([]byte, 65535)
		toP2Bytes, err = io.CopyBuffer(p2, p1, buf)
		if err != nil {
			log.Debug(err)
		}
		sync <- 1
	}()

	<-sync
	select {
	case <-sync:
	case <-time.After(time.Second * 10):
	}
	time.Sleep(time.Second * 1)
	p1.Close()
	p2.Close()
	return toP1Bytes, toP2Bytes
}

//io.Copy version
func trans(p1, p2 io.ReadWriteCloser) (int64, int64) {
	var sync = make(chan int64, 2)
	var toP1Bytes, toP2Bytes int64
	var err error
	go func() {
		toP1Bytes, err = io.Copy(p1, p2)
		if err != nil {
			log.Debug(err)
		}
		sync <- 1
	}()

	go func() {
		toP2Bytes, err = io.Copy(p2, p1)
		if err != nil {
			log.Debug(err)
		}
		sync <- 1
	}()

	<-sync
	select {
	case <-sync:
	case <-time.After(time.Second * 10):
	}
	p1.Close()
	p2.Close()
	return toP1Bytes, toP2Bytes
}

func (p *Proxy) forward(inConn io.ReadWriteCloser, raddr net.Addr) {
	ip := strings.Split(raddr.String(), ":")[0]
	var buf = make([]byte, 512)
	if n, e := inConn.Read(buf); e != nil {
		log.Println(e)
	} else {
		buf = buf[:n]
		peek := peektype.NewPeek()
		peek.SetBuf(buf)
		t := peek.Parse()

		var remotes []string
		var hostname = peek.Hostname
		switch t {
		case peektype.SSH:
			remotes = p.getRemotes("SSH", "", ip)
		case peektype.HTTP:
			remotes = p.getRemotes("HTTP", peek.Hostname, ip)
			log.Println("peekhost:", hostname)
		case peektype.HTTPS:
			remotes = p.getRemotes("HTTPS", peek.Hostname, ip)
			log.Println("peekhost:", hostname)
		case peektype.UNKNOWN:
			remotes = p.getRemotes("TCP", peek.Hostname, ip)
		}

		if len(remotes) == 0 {
			log.Warning("Unable to find remote hosts for", ip, hostname)
			inConn.Close()
		} else { //get remotes
			var bConn net.Conn
			var err error
			for i, remote := range remotes {
				if bConn, err = net.DialTimeout("tcp", remote, time.Millisecond*400); err != nil {
					log.Println(remote, err)
					continue
				} else {
					if i > 0 {
						log.Warning("swap first remote:", remotes[0], "with", remotes[i])
						remotes[0], remotes[i] = remotes[i], remotes[0]
					}
					log.Println(raddr, "->", bConn.RemoteAddr())
					nn, _ := bConn.Write(buf)
					out, in := trans3(inConn, bConn)
					in += int64(nn)
					user := hostname
					if tf, ok := p.ut[user]; !ok {
						p.ut[user] = &Taf{}
						p.ut[user].ip = ip
					} else {
						tf.in += uint64(in)
						tf.out += uint64(out)
					}
					log.Infof("%s(%s->%s) in:%d out:%d\n", hostname, raddr.String(), remote, in, out)
					return
				}
				log.Warning("All backend servers are die!", remotes)
				inConn.Close()
			}
		}
	}
}

func (p *Proxy) Start() {
	if p.sendTraf {
		go p.autoSentTraf(time.Minute * 2)
	}
}

var LIM = limit.NewLIMIT()

func (p *Proxy) listenAndProxy(listenAddr string) {
	go kcpp.ListenKCP(listenAddr, p.forward)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Error("Failed to setup listener:", err)
		return
	} else {
		log.Info("Listen on", listenAddr)
		p.listeners[listenAddr] = listener
	}

	for {
		if conn, err := listener.Accept(); err != nil {
			log.Println(err)
		} else {
			go p.forward(conn, conn.RemoteAddr())
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
					sendTraf.SendTraf(u, t.ip, p.addByteUrl, p.name, uint64(t.in), uint64(t.out))
				} else {
					sendTraf.SendTraf(u, t.ip, p.addByteUrl, p.name, 0, 0)
				}
				log.Info(t.ip, u, humanize.Bytes(uint64(float64(t.out)/time.Now().Sub(from).Seconds())), "/s↓",
					humanize.Bytes(uint64(float64(t.in)/time.Now().Sub(from).Seconds())), "/s↑")
				t.out = 0
				t.in = 0
			}
		}
		from = time.Now()
	}
}
