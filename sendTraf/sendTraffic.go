package sendTraf

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/linexjlin/simple-log"
)

type Traf struct {
	host, ip string
	server   string
	in, out  uint64
	url      string
	timeOut  time.Time
}

func (t *Traf) AddTraf(in, out uint64) {
	t.in += in
	t.out += out
	if time.Now().After(t.timeOut) {
		go t.SendTraf()
	}
}

func (t *Traf) ResetTraf() {
	t.in, t.out = 0, 0
}

func (t *Traf) SetIP(ip string) {
	t.ip = ip
}

func (t *Traf) SendTraf() {
	if t.url == "" {
		return
	}
	if req, err := http.NewRequest("GET", t.url, nil); err != nil {
		log.Warning(err)
		return
	} else {
		q := req.URL.Query()
		q.Add("user", t.host)
		q.Add("userIP", t.ip)
		q.Add("server", t.server)
		q.Add("in", fmt.Sprint(t.in))
		q.Add("out", fmt.Sprint(t.out))
		req.URL.RawQuery = q.Encode()
		client := http.Client{Timeout: time.Second * 15}
		_, err := client.Do(req)
		if err != nil {
			log.Error(err, *t)
		} else {
			t.ResetTraf()
		}
	}
	t.timeOut = time.Now().Add(time.Minute * 2)
}

type Trafs struct {
	tfs         map[string]*Traf
	url, server string
	lock        sync.Mutex
}

func (ts *Trafs) GetTraf(host string) *Traf {
	ts.lock.Lock()
	defer ts.lock.Unlock()
	return ts.tfs[host]
}

func (ts *Trafs) Exist(host string) bool {
	ts.lock.Lock()
	defer ts.lock.Unlock()
	if _, ok := ts.tfs[host]; !ok {
		return false
	}
	return true
}

func (ts *Trafs) Add(host string) {
	ts.lock.Lock()
	defer ts.lock.Unlock()
	ts.tfs[host] = &Traf{host: host, url: ts.url, server: ts.server, timeOut: time.Now()}
}

func (ts *Trafs) AddTraf(host, userIP, server string, in, out uint64) {
	if !ts.Exist(host) {
		ts.Add(host)
	}
	ts.server = server
	tf := ts.GetTraf(host)
	tf.SetIP(userIP)
	tf.AddTraf(in, out)
}

var Tfs *Trafs

func init() {
	Tfs = &Trafs{
		tfs: make(map[string]*Traf),
		url: "https://scdn.linkown.com/addTraffic",
	}
}
