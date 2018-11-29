package main

import (
	"net/http"
	"time"

	update "github.com/inconshreveable/go-update"
	"github.com/linexjlin/simple-log"
)

func updater(url string) {
	for {
		log.Info("Update program via:", url)
		if err := doUpdate(url); err != nil {
			log.Warning("Update fail", err)
		}
		log.Info("Update Success")
		time.Sleep(time.Hour * 1)
	}
}

func doUpdate(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	err = update.Apply(resp.Body, update.Options{})
	if err != nil {
		log.Error("update error", err)
	}
	return err
}
