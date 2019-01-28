package sendTraf

import (
	"fmt"
	"net/http"
	"time"

	"github.com/linexjlin/simple-log"
)

func SendTraf(user, userIP, url, server string, in, out uint64) {
	http.DefaultClient.Timeout = time.Minute * 3
	if url == "" {
		return
	}
	if req, err := http.NewRequest("GET", url, nil); err != nil {
		log.Warning(err)
		return
	} else {
		q := req.URL.Query()
		q.Add("user", user)
		q.Add("userIP", userIP)
		q.Add("server", server)
		q.Add("in", fmt.Sprint(in))
		q.Add("out", fmt.Sprint(out))
		req.URL.RawQuery = q.Encode()
		client := http.Client{}
		if rsp, err := client.Do(req); err != nil {
			log.Warning(err)
		} else {
			rsp.Body.Close()
		}
	}
}
