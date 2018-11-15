package main

import (
	"testing"
)

func TestGetConfig(t *testing.T) {
	if config, err := getConfig("http://127.0.0.1:8035/config?server=LA83"); err != nil {
		t.Fail()
	} else {
		t.Log(config)
	}
}

func TestConfig2route(t *testing.T) {
	if config, err := getConfig("http://127.0.0.1:8035/config?server=LA83"); err != nil {
		t.Fail()
	} else {
		t.Log(config)
		r := config2route(&config)
		t.Log(r)
	}
}
