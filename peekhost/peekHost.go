package peekhost

import (
	"errors"
	"net"
	"strings"
)

func PeekHost(conn net.Conn) (buf []byte, host string, err error) {
	return peekHostSimple(conn)
}

func peekHostSimple(conn net.Conn) (buf []byte, host string, err error) {
	b := make([]byte, 128)
	if n, err := conn.Read(b); err != nil {
		return []byte{}, "", err
	} else {
		b = b[:n]
	}

	datStr := string(b)
	lines := strings.Split(datStr, "\n")
	if len(lines) > 2 {
		if strings.Contains(lines[1], "Host") {
			sl := strings.Split(lines[1], ":")
			if len(sl) > 1 {
				return b, strings.TrimSpace(sl[1]), nil
			}
		}
	}
	return b, "", errors.New("Unable to find host")
}
