package peektype

import (
	"log"
	"net"
	"testing"
)

func TestListenServe(t *testing.T) {
	listenAddr := "0.0.0.0:58080"
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("Failed to setup listener: %v", err)
	} else {
		log.Println("Listen on", listenAddr)
	}

	for {
		if conn, err := listener.Accept(); err != nil {
			log.Println(err)
		} else {
			log.Println("accept connectino from:", conn.RemoteAddr())
			go func() {
				dat, cType, res, err := PeekType(conn)
				log.Println("")
				log.Println("dat:", string(dat))
				log.Println("")
				log.Println(res.(string))
				log.Println(err)
				log.Println("peeked host:", cType)
			}()
		}
	}
}
