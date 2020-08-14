package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/linexjlin/simple-log"
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
	Type   string
	HTTP   HttpHttps
	Group  ConfigServiceGroup `json:",omitempty"`
}

type Config struct {
	Listen      ConfigServiceGroup  `json:",omitempty"`
	HTTP        HttpHttps           `json:",omitempty"`
	UnknownHTTP HttpHttps           `json:",omitempty"`
	SSH         ConfigServiceGroup  `json:",omitempty"`
	Unknown     ConfigServiceGroup  `json:",omitempty"`
	Hosts       map[string]HostInfo `json:",omitempty"`
	Hash        string
}

const (
	TMPConfig = "/tmp/.config.json"
)

func getConfig(url, server, hash string) (Config, error) {
	var config Config
	var err error
	http.DefaultClient.Timeout = time.Minute * 3

	if req, err := http.NewRequest("GET", url, nil); err != nil {
		log.Println(err)
		return LoadConfigFromFile("/tmp/.config.json")
	} else {
		q := req.URL.Query()
		q.Add("server", server)
		q.Add("hash", hash)
		req.URL.RawQuery = q.Encode()
		client := http.Client{}
		if rsp, err := client.Do(req); err != nil {
			log.Println(err)
			return config, err
		} else {
			if rsp.StatusCode == 200 {
				if dat, err := ioutil.ReadAll(rsp.Body); err != nil {
					log.Error(err)
				} else {
					if err = json.Unmarshal(dat, &config); err != nil {
						log.Error(err)
					} else {
						if len(config.Listen.Services) == 0 {
							return config, errors.New("No Listen found!")
						} else {
							ioutil.WriteFile(TMPConfig, dat, 0644)
							log.Println("Load config success!")
						}
					}
				}
			} else {
				log.Error(rsp.StatusCode)
				log.Error(req.URL.Host, req.URL.RequestURI())
				return config, errors.New("No Found!")
			}
			rsp.Body.Close()
		}
	}

	return config, err
}

func LoadConfigFromFile(path string) (Config, error) {
	var config Config
	var err error
	log.Notice("Load config from", TMPConfig)
	if dat, err := ioutil.ReadFile(path); err != nil {
		log.Error(err)
	} else {
		if err = json.Unmarshal(dat, &config); err != nil {
			log.Error(err)
		}
	}
	return config, err
}

func config2route(c *Config) *tp.Route {
	r := tp.NewRoute()

	if c.Listen.ServiceGroup != nil {
		var ss []string
		for _, s := range c.Listen.Services {
			ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
		}
		r.Add(tp.LISTEN, "LISTEN", 0, "", "", ss)
	}

	if c.HTTP.HTTP.ServiceGroup != nil {
		var ss []string
		for _, s := range c.HTTP.HTTP.Services {
			ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
		}
		r.Add(tp.NHTTP, "", int(c.HTTP.HTTP.MaxIP), c.HTTP.HTTP.Policy, "", ss)
	}

	if c.HTTP.HTTPS.ServiceGroup != nil {
		var ss []string
		for _, s := range c.HTTP.HTTPS.Services {
			ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
		}
		r.Add(tp.NHTTPS, "", int(c.HTTP.HTTPS.MaxIP), c.HTTP.HTTPS.Policy, "", ss)
	}

	if c.UnknownHTTP.HTTP.ServiceGroup != nil {
		var ss []string
		for _, s := range c.UnknownHTTP.HTTP.Services {
			ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
		}
		r.Add(tp.FHTTP, "", int(c.UnknownHTTP.HTTP.MaxIP), c.UnknownHTTP.HTTP.Policy, "", ss)
	}

	if c.UnknownHTTP.HTTPS.ServiceGroup != nil {
		var ss []string
		for _, s := range c.UnknownHTTP.HTTPS.Services {
			ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
		}
		r.Add(tp.FHTTPS, "", int(c.UnknownHTTP.HTTPS.MaxIP), c.UnknownHTTP.HTTPS.Policy, "", ss)
	}

	if c.SSH.ServiceGroup != nil {
		var ss []string
		for _, s := range c.SSH.Services {
			ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
		}
		r.Add(tp.SSH, "", 0, c.SSH.Policy, "", ss)
	}

	if c.Unknown.ServiceGroup != nil {
		var ss []string
		for _, s := range c.Unknown.Services {
			ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
		}
		r.Add(tp.UNKNOWN, "UNKNOWN", 0, c.Unknown.Policy, "", ss)
	}

	for name, h := range c.Hosts {
		if h.HTTP.HTTP.ServiceGroup != nil {
			var ss []string
			for _, s := range h.HTTP.HTTP.Services {
				ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
			}
			var maxIP int
			if h.MaxIP > h.HTTP.HTTP.MaxIP {
				maxIP = int(h.MaxIP)
			} else {
				maxIP = int(h.HTTP.HTTP.MaxIP)
			}
			r.Add(tp.UHTTP, name, maxIP, h.HTTP.HTTP.Policy, h.Type, ss)
		} else {
			var ss []string
			r.Add(tp.UHTTP, name, int(h.MaxIP), h.Policy, h.Type, ss)
		}

		if h.HTTP.HTTPS.ServiceGroup != nil {
			var ss []string
			for _, s := range h.HTTP.HTTPS.Services {
				ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
			}
			var maxIP int
			if h.MaxIP > h.HTTP.HTTPS.MaxIP {
				maxIP = int(h.MaxIP)
			} else {
				maxIP = int(h.HTTP.HTTPS.MaxIP)
			}
			r.Add(tp.UHTTPS, name, maxIP, h.HTTP.HTTPS.Policy, h.Type, ss)
		} else {
			var ss []string
			r.Add(tp.UHTTPS, name, int(h.MaxIP), h.Policy, h.Type, ss)
		}
		if h.Group.ServiceGroup != nil {
			var ss []string
			for _, s := range h.Group.Services {
				ss = append(ss, fmt.Sprintf("%s:%d", s.IP, s.Port))
			}
			var maxIP int
			if h.MaxIP > h.Group.MaxIP {
				maxIP = int(h.MaxIP)
			} else {
				maxIP = int(h.Group.MaxIP)
			}
			switch h.Type {
			case "port":
				r.Add(tp.PORT, name, maxIP, h.Group.Policy, h.Type, ss)
			case "lip":
				r.Add(tp.LIP, name, maxIP, h.Group.Policy, h.Type, ss)
			case "ipport":
				r.Add(tp.IPPORT, name, maxIP, h.Group.Policy, h.Type, ss)
			default:
				r.Add(tp.UNKNOWN, name, maxIP, h.Group.Policy, h.Type, ss)
			}
		}
	}

	return r
}

func autoUpdateConfig(url, server string) {
	var hash string
	var r *tp.Route
	var lastUpdate time.Time
	for {
		if config, err := getConfig(url, server, hash); err != nil {
			log.Println(err)
		} else {
			hashNew := config.Hash
			log.Println("Config Hash:", hashNew)
			if hashNew != hash {
				r = config2route(&config)
				//r.OptimizeBackend()
				P.SetRoute(r)
				hash = hashNew
				lastUpdate = time.Now()
			}
		}
		//Continually to optmize backend servers
		if time.Now().Sub(lastUpdate) > time.Minute*15 {
			r.OptimizeBackend()
			lastUpdate = time.Now()
		}
		time.Sleep(time.Minute * 1)
	}
}
