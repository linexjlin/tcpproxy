package limit

import (
	"sync"
	"time"
)

func newConn(max int) *CONN {
	var conn CONN
	conn.Init(max)
	return &conn

}

var mux sync.Mutex

type CONN struct {
	dat      map[string]time.Time
	cnt, max int
}

func (c *CONN) Init(max int) {
	c.dat = make(map[string]time.Time)
}

func (c *CONN) ChangeMax(max int) {
	if c.max != max {
		c.max = max
	}
}

func (c *CONN) addIP(ip string) {
	c.dat[ip] = time.Now()
}

func (c *CONN) CheckAndAdd(ip string) bool {
	if _, ok := c.dat[ip]; ok {
		mux.Lock()
		c.dat[ip] = time.Now()
		mux.Unlock()
		return true
	} else {
		if len(c.dat) < c.max {
			c.addIP(ip)
			return true
		} else {
			for i, t := range c.dat {
				if !c.isActive(t) {
					delete(c.dat, i)
					c.addIP(ip)
					return true
				}
			}
			return false
		}
	}
}

func (c *CONN) isActive(t time.Time) bool {
	return time.Now().Sub(t).Seconds() < 60
}

type LIMIT struct {
	dat map[string]CONN
}

func NewLIMIT() *LIMIT {
	var l LIMIT
	l.Init()
	return &l
}

func (l *LIMIT) Init() {
	l.dat = make(map[string]CONN)

}

func (l *LIMIT) Check(u, ip string, maxIP int) bool {
	if maxIP == 0 {
		return true
	}
	if c, ok := l.dat[u]; !ok {
		var con = newConn(maxIP)
		con.CheckAndAdd(ip)
		l.dat[u] = (*con)
		return true
	} else {
		c.ChangeMax(maxIP)
		return c.CheckAndAdd(ip)
	}
}
