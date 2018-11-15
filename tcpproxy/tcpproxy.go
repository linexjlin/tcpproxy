package tcpproxy

import (
	"io"
	"log"
	"net"
	"regexp"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/linexjlin/peektype"
	limit "github.com/linexjlin/tcpproxy/limitip"
	"github.com/linexjlin/tcpproxy/sendTraf"
)

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

func iocopy(w io.Writer, r io.Reader) (int64, error) {
	buf := make([]byte, 1)
	var s int64
	for {
		if n, err := r.Read(buf); err != nil {
			return s, err
		} else {
			w.Write(buf[:n-1])
			s += int64(n)
		}
	}
}

func NewProxy() *Proxy {
	p := Proxy{}
	p.ut = make(map[string]*Taf)
	return &p
}

func (p *Proxy) UpdateConfig(new *Config) {
	new.regRoute = make(map[*regexp.Regexp][]string)
	for rs, v := range new.Route {
		if len(v) > 0 {
			r := regexp.MustCompile(rs)
			new.regRoute[r] = v
		}
	}
	p.cfg = new
}

func (p *Proxy) GetConfig() *Config {
	return p.cfg
}

func (p *Proxy) getRemotes(rType, host string) []string {
	config := p.GetConfig()
	switch rType {
	case "HTTP", "HTTPS":
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

func (p *Proxy) Start() {
	p.listenAndProxyAll()
	if p.SendTraf {
		go p.AutoSentTraf(time.Minute * 2)
	}
}

func (p *Proxy) listenAndProxyAll() {
	config := p.GetConfig()
	for _, addr := range config.Listen {
		go p.listenAndProxy(addr)
	}
}

var LIM = limit.NewLIMIT()

func (p *Proxy) listenAndProxy(listenAddr string) {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Println("Failed to setup listener:", err)
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
