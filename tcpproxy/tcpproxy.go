package tcpproxy

import (
	"io"
	"net"
	"strings"
	"sync"
	"time"

	//	humanize "github.com/dustin/go-humanize"
	"github.com/linexjlin/peektype"
	"github.com/linexjlin/simple-log"
	//"github.com/linexjlin/tcpproxy/kcpp"
	"github.com/eternnoir/gncp"
	limit "github.com/linexjlin/tcpproxy/limitip"
	"github.com/linexjlin/tcpproxy/sendTraf"
	tl "github.com/linexjlin/tcpproxy/tcplatency"
	"golang.org/x/net/proxy"
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
	PORT
	LIP
	IPPORT
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
	btype    string
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

func (r *Route) Add(rtype int, name string, maxIP int, policy, btype string, services []string) {
	var rule Rule
	rule.name = name
	rule.rtype = rtype

	var backend Backend
	backend.maxIP = maxIP
	backend.services = services
	backend.policy = policy
	backend.btype = btype
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
	connMap                    sync.Map
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

func (p *Proxy) getRemotesByAddr(ip string, laddr, raddr net.Addr) (int, []string) {
	var t int
	//Try Port; eg: 50000
	host := strings.Split(laddr.String(), ":")[1]
	t = PORT
	b, ok := p.route.rules[Rule{PORT, host}]

	if !ok {
		//try local ip; eg: 1.2.3.4
		host = strings.Split(laddr.String(), ":")[0]
		t = LIP
		b, ok = p.route.rules[Rule{LIP, host}]
	}
	if !ok {
		//try local IP + PORT match 1.2.4.3:50000
		host = laddr.String()
		t = IPPORT
		b, ok = p.route.rules[Rule{IPPORT, host}]
	}
	if !ok {
		return UNKNOWN, []string{}
	}

	if LIM.Check(host, ip, b.maxIP) {
		if len(b.services) > 0 {
			log.Println("UserRoute")
			return t, b.services
		}
	} else {
		log.Warning("Max IP reached", host, ip, b.maxIP)
	}
	return UNKNOWN, []string{}
}

func (p *Proxy) getRemotes(rType, host, ip string, laddr, raddr net.Addr) (int, []string) {
	switch rType {
	case "HTTP":
		b, ok := p.route.rules[Rule{UHTTP, host}]
		if ok {
			if LIM.Check(host, ip, b.maxIP) {
				if len(b.services) > 0 {
					log.Println("User HTTP")
					return UHTTP, b.services
				} else {
					log.Println("System HTTP Backends")
					return NHTTP, p.route.rules[Rule{NHTTP, ""}].services
				}
			} else {
				log.Warning("Max IP reached", host, ip, b.maxIP)
			}
		}
		if t, b := p.getRemotesByAddr(ip, laddr, raddr); len(b) > 0 {
			return t, b
		}
		log.Println("Unknown HTTP Backends", host)
		return FHTTP, p.route.rules[Rule{FHTTP, ""}].services
	case "HTTPS":
		b, ok := p.route.rules[Rule{UHTTPS, host}]
		if ok {
			if LIM.Check(host, ip, b.maxIP) {
				if len(b.services) > 0 {
					log.Println("User HTTPS")
					return UHTTPS, b.services
				} else {
					log.Println("System HTTPS Backends")
					return NHTTPS, p.route.rules[Rule{NHTTPS, ""}].services
				}
			} else {
				log.Warning("Max IP reached", host, ip, b.maxIP)
			}
		}
		if t, b := p.getRemotesByAddr(ip, laddr, raddr); len(b) > 0 {
			return t, b
		}
		log.Println("Unknown HTTPS Backends", host)
		return FHTTPS, p.route.rules[Rule{FHTTPS, ""}].services
	case "SSH":
		host = "SSH"
		if b, ok := p.route.rules[Rule{SSH, ""}]; ok {
			if LIM.Check(host, ip, b.maxIP) {
				if len(b.services) > 0 {
					log.Println("UserRoute")
					return SSH, b.services
				}
			} else {
				log.Warning("Max IP reached", host, ip, b.maxIP)
			}

		}
		if t, b := p.getRemotesByAddr(ip, laddr, raddr); len(b) > 0 {
			return t, b
		}
		host = "UNKNOWN"
		return UNKNOWN, p.route.rules[Rule{UNKNOWN, ""}].services
	case "TCP":
		return p.getRemotesByAddr(ip, laddr, raddr)
	default:
		host = "UNKNOWN"
		return UNKNOWN, p.route.rules[Rule{UNKNOWN, ""}].services
	}
	return UNKNOWN, []string{}
}

var lPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 32*1024)
	},
}

//io.CopyBuffer
func trans(p1, p2 io.ReadWriteCloser) (int64, int64) {
	var sync = make(chan int, 2)
	var toP1Bytes, toP2Bytes int64
	var err error
	go func() {
		buf := lPool.Get().([]byte)
		toP1Bytes, err = io.CopyBuffer(p1, p2, buf)
		if err != nil {
			log.Debug(err)
		}
		lPool.Put(buf)
		sync <- 1
	}()

	go func() {
		buf := lPool.Get().([]byte)
		toP2Bytes, err = io.CopyBuffer(p2, p1, buf)
		if err != nil {
			log.Debug(err)
		}
		lPool.Put(buf)
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

var Socks5 string

func (p *Proxy) forwarder(inConn io.ReadWriteCloser, laddr, raddr net.Addr) {
	defer func() {
		recover()
	}()
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
		var rt int
		switch t {
		case peektype.SSH:
			rt, remotes = p.getRemotes("SSH", "", ip, laddr, raddr)
			log.Infof("SSH: %s\n", raddr.String())
		case peektype.HTTP:
			rt, remotes = p.getRemotes("HTTP", peek.Hostname, ip, laddr, raddr)
			log.Debugf("http://%s %s\n", hostname, raddr.String())
		case peektype.HTTPS:
			rt, remotes = p.getRemotes("HTTPS", peek.Hostname, ip, laddr, raddr)
			log.Infof("https://%s %s\n", hostname, raddr.String())
		case peektype.UNKNOWN:
			rt, remotes = p.getRemotes("TCP", peek.Hostname, ip, laddr, raddr)
			log.Infof("UNKNOWN: %s\n", raddr.String())
		}

		if len(remotes) == 0 {
			log.Warning("Unable to find remote hosts for", ip, hostname)
			inConn.Close()
		} else { //get remotes
			var bConn net.Conn
			var err error
			var pool *gncp.GncpPool
			for i, remote := range remotes {
				if v, ok := p.connMap.Load(remote); ok {
					log.Debug("Hit cache for", remote)
					pool = v.(*gncp.GncpPool)
				} else {
					log.Debug("create new pool for", remote)
					pool, err = gncp.NewPool(3, 10, func() (net.Conn, error) {
						if Socks5 != "" {
							dial, err := proxy.SOCKS5("tcp", Socks5, nil, proxy.Direct)
							if err != nil {
								log.Error(err)
							}
							return dial.Dial("tcp", remote)
						} else {
							return net.Dial("tcp", remote)
						}
					})
					p.connMap.Store(remote, pool)
					if err != nil {
						log.Error(err)
						continue
					}
				}
				if bConn, err = pool.GetWithTimeout(time.Millisecond * 100); err != nil {
					log.Println(remote, err)
					continue
				} else {
					if i > 0 {
						log.Warning("swap first remote:", remotes[0], "with", remotes[i])
						remotes[0], remotes[i] = remotes[i], remotes[0]
					}
					log.Println(raddr, "->", bConn.RemoteAddr())
					nn, _ := bConn.Write(buf)
					out, in := trans(inConn, bConn)
					in += int64(nn)
					defer pool.Remove(bConn)
					var user string
					switch rt {
					case IPPORT:
						user = laddr.String()
					case LIP:
						user = strings.Split(laddr.String(), ":")[1]
					case PORT:
						user = strings.Split(laddr.String(), ":")[1]
					default:
						user = hostname
					}

					sendTraf.Tfs.AddTraf(user, ip, p.name, uint64(in), uint64(out))

					/*
						if tf, ok := p.ut[user]; !ok {
							p.ut[user] = &Taf{}
							p.ut[user].ip = ip
						} else {
							tf.in += uint64(in)
							tf.out += uint64(out)
						}*/

					log.Infof("%s [%s->%s->%s] [I:%d O:%d]\n", user, raddr.String(), laddr.String(), remote, in, out)
					return
				}
				log.Warning("All backend servers are die!", remotes)
				inConn.Close()
			}
		}
	}
}

/*
func (p *Proxy) Start() {
	if p.sendTraf {
		go p.autoSentTraf(time.Minute * 2)
	}
}*/

var LIM = limit.NewLIMIT()

func (p *Proxy) listenAndProxy(listenAddr string) {
	//go kcpp.ListenKCP(listenAddr, p.forwarder)
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
			conn.SetDeadline(time.Now().Add(time.Hour))
			go p.forwarder(conn, conn.LocalAddr(), conn.RemoteAddr())
		}
	}
}

/*
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
} */
