package sendTraf

import (
	"fmt"
	"log"
	"net/http"
)

func SendTraf(user, userIP, url string, bytes uint64) {
	if url == "" {
		return
	}
	if req, err := http.NewRequest("GET", url, nil); err != nil {
		log.Println(err)
		return
	} else {
		q := req.URL.Query()
		q.Add("user", user)
		q.Add("userIP", userIP)
		q.Add("byte", fmt.Sprint(bytes))
		req.URL.RawQuery = q.Encode()
		client := http.Client{}
		if rsp, err := client.Do(req); err != nil {
			log.Println(err)
			rsp.Body.Close()
		} else {
			rsp.Body.Close()
		}
	}
}
