package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	tp "github.com/linexjlin/tcpproxy/tcpproxy"
)

type ServiceGroup struct {
	Name   string `json:",omitempty"`
	Policy string `json:",omitempty"`
	MaxIP  uint   `json:",omitempty"`
}

type Service struct {
	ServiceGroupID uint   `gorm:"index" json:",omitempty"`
	IP             string `gorm:"index" json:",omitempty"`
	Port           uint   `json:",omitempty"`
}

type ConfigServiceGroup struct {
	*ServiceGroup `json:",omitempty"`
	Services      []Service `json:",omitempty"`
}

type HttpHttps struct {
	HTTP  ConfigServiceGroup `json:",omitempty"`
	HTTPS ConfigServiceGroup `json:",omitempty"`
}

type HostInfo struct {
	MaxIP  uint
	Policy string
	HTTP   HttpHttps
}

type Config struct {
	Listen      ConfigServiceGroup  `json:",omitempty"`
	HTTP        HttpHttps           `json:",omitempty"`
	UnknownHTTP HttpHttps           `json:",omitempty"`
	SSH         ConfigServiceGroup  `json:",omitempty"`
	Unknown     ConfigServiceGroup  `json:",omitempty"`
	Hosts       map[string]HostInfo `json:",omitempty"`
}

func getConfig(url string) (Config, error) {
	var config Config
	if rsp, err := http.Get(url); err != nil {
		log.Println(err)
		return config, err
	} else {
		if err = json.NewDecoder(rsp.Body).Decode(&config); err != nil {
			log.Println(err)
			return config, err
		} else {
			log.Println("Load config success!")
			rsp.Body.Close()
		}
	}
	return config, nil
}

func config2route(c *Config) *tp.Route {
	r := tp.NewRoute()

	if c.Listen.ServiceGroup != nil {
		var ss []string
		for _, s := range c.Listen.Services {
			ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
		}
		r.Add(tp.LISTEN, "LISTEN", 0, "", ss)
	}

	if c.HTTP.HTTP.ServiceGroup != nil {
		var ss []string
		for _, s := range c.HTTP.HTTP.Services {
			ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
		}
		r.Add(tp.NHTTP, "", int(c.HTTP.HTTP.MaxIP), c.HTTP.HTTP.Policy, ss)
	}

	if c.HTTP.HTTPS.ServiceGroup != nil {
		var ss []string
		for _, s := range c.HTTP.HTTPS.Services {
			ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
		}
		r.Add(tp.NHTTPS, "", int(c.HTTP.HTTPS.MaxIP), c.HTTP.HTTPS.Policy, ss)
	}

	if c.UnknownHTTP.HTTP.ServiceGroup != nil {
		var ss []string
		for _, s := range c.UnknownHTTP.HTTP.Services {
			ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
		}
		r.Add(tp.FHTTP, "", int(c.UnknownHTTP.HTTP.MaxIP), c.UnknownHTTP.HTTP.Policy, ss)
	}

	if c.UnknownHTTP.HTTPS.ServiceGroup != nil {
		var ss []string
		for _, s := range c.UnknownHTTP.HTTPS.Services {
			ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
		}
		r.Add(tp.FHTTPS, "", int(c.UnknownHTTP.HTTPS.MaxIP), c.UnknownHTTP.HTTPS.Policy, ss)
	}

	if c.SSH.ServiceGroup != nil {
		var ss []string
		for _, s := range c.SSH.Services {
			ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
		}
		r.Add(tp.SSH, "", 0, c.SSH.Policy, ss)
	}

	if c.Unknown.ServiceGroup != nil {
		var ss []string
		for _, s := range c.Unknown.Services {
			ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
		}
		r.Add(tp.UNKNOWN, "UNKNOWN", 0, c.Unknown.Policy, ss)
	}

	for name, h := range c.Hosts {
		if h.HTTP.HTTP.ServiceGroup != nil {
			var ss []string
			for _, s := range h.HTTP.HTTP.Services {
				ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
			}
			r.Add(tp.UHTTP, name, int(h.HTTP.HTTP.MaxIP), h.HTTP.HTTP.Policy, ss)
		} else {
			var ss []string
			r.Add(tp.UHTTP, name, int(h.MaxIP), h.Policy, ss)
		}

		if h.HTTP.HTTPS.ServiceGroup != nil {
			var ss []string
			for _, s := range h.HTTP.HTTPS.Services {
				ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
			}
			r.Add(tp.UHTTPS, name, int(h.HTTP.HTTPS.MaxIP), h.HTTP.HTTPS.Policy, ss)
		} else {
			var ss []string
			r.Add(tp.UHTTPS, name, int(h.MaxIP), h.Policy, ss)
		}
	}

	return r
}

func autoUpdateConfig(url string) {
	var hash string
	h := sha256.New()
	for {
		if config, err := getConfig(url); err != nil {
			log.Println(err)
		} else {
			dat, _ := json.Marshal(config)
			h.Write(dat)
			hashed := h.Sum(nil)
			hashNew := hex.EncodeToString(hashed)
			h.Reset()
			log.Println("Config Hash:", hashNew)
			if hashNew != hash {
				r := config2route(&config)
				r.OptimizeBackend()
				P.SetRoute(r)
				hash = hashNew
			}
		}
		time.Sleep(time.Minute * 1)
	}
}
