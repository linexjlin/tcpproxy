package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

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

type HttpHttps struct {
	HTTP  ConfigServiceGroup `json:",omitempty"`
	HTTPS ConfigServiceGroup `json:",omitempty"`
}

type ConfigServiceGroup struct {
	*ServiceGroup `json:",omitempty"`
	Services      []Service `json:",omitempty"`
}

type Config struct {
	Listen      ConfigServiceGroup   `json:",omitempty"`
	HTTP        HttpHttps            `json:",omitempty"`
	UnknownHTTP HttpHttps            `json:",omitempty"`
	SSH         ConfigServiceGroup   `json:",omitempty"`
	Unknown     ConfigServiceGroup   `json:",omitempty"`
	Hosts       map[string]HttpHttps `json:",omitempty"`
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
	var ss []string

	if c.Listen.ServiceGroup != nil {
		for _, s := range c.Listen.Services {
			ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
		}
		r.Add(tp.LISTEN, "LISTEN", 0, "", ss)
	}

	if c.HTTP.HTTP.ServiceGroup != nil {
		ss = ss[:0]
		for _, s := range c.HTTP.HTTP.Services {
			ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
		}
		r.Add(tp.NHTTP, c.HTTP.HTTP.Name, int(c.HTTP.HTTP.MaxIP), c.HTTP.HTTP.Policy, ss)
	}

	if c.HTTP.HTTPS.ServiceGroup != nil {
		ss = ss[:0]
		for _, s := range c.HTTP.HTTPS.Services {
			ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
		}
		r.Add(tp.NHTTPS, c.HTTP.HTTPS.Name, int(c.HTTP.HTTPS.MaxIP), c.HTTP.HTTPS.Policy, ss)
	}

	if c.UnknownHTTP.HTTP.ServiceGroup != nil {
		ss = ss[:0]
		for _, s := range c.UnknownHTTP.HTTP.Services {
			ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
		}
		r.Add(tp.FHTTP, c.UnknownHTTP.HTTP.Name, int(c.UnknownHTTP.HTTP.MaxIP), c.UnknownHTTP.HTTP.Policy, ss)
	}

	if c.UnknownHTTP.HTTPS.ServiceGroup != nil {
		ss = ss[:0]
		for _, s := range c.UnknownHTTP.HTTPS.Services {
			ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
		}
		r.Add(tp.FHTTPS, c.UnknownHTTP.HTTPS.Name, int(c.UnknownHTTP.HTTPS.MaxIP), c.UnknownHTTP.HTTPS.Policy, ss)
	}

	if c.SSH.ServiceGroup != nil {
		ss = ss[:0]
		for _, s := range c.SSH.Services {
			ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
		}
		r.Add(tp.SSH, "SSH", 0, c.SSH.Policy, ss)
	}

	if c.Unknown.ServiceGroup != nil {
		ss = ss[:0]
		for _, s := range c.Unknown.Services {
			ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
		}
		r.Add(tp.UNKNOWN, "UNKNOWN", 0, c.Unknown.Policy, ss)
	}

	for _, h := range c.Hosts {
		if h.HTTP.ServiceGroup != nil {
			ss = ss[:0]
			for _, s := range h.HTTP.Services {
				ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
			}
			r.Add(tp.UHTTP, h.HTTP.Name, int(h.HTTP.MaxIP), h.HTTP.Policy, ss)
		}

		if h.HTTPS.ServiceGroup != nil {
			ss = ss[:0]
			for _, s := range h.HTTPS.Services {
				ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
			}
			r.Add(tp.UHTTPS, h.HTTPS.Name, int(h.HTTPS.MaxIP), h.HTTPS.Policy, ss)
		}
	}

	return r
}
