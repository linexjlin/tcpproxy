package peektype

import (
	"io"
	"net"
	"strings"
)

const (
	HTTP = iota
	SSH
	NORMALTCP
	UNKNOWN
)
const (
	HTTPPATTAN = "GET HEAD POST POST PUT DELE TRAC OPTI CONN PATC"
	SSHPATTAN  = "SSH-"
)

func PeekType(conn net.Conn) (buf []byte, cType int, res interface{}, err error) {
	return peek(conn)
}

func peek(r io.Reader) (buf []byte, cType int, res interface{}, err error) {
	header := make([]byte, 4)
	if n, err := r.Read(header); err != nil {
		return []byte{}, UNKNOWN, "", err
	} else {
		header = header[:n]
	}
	buf = append(buf, header...)
	switch {
	case strings.Contains(SSHPATTAN, string(header)):
		return buf, SSH, "", nil
	case strings.Contains(HTTPPATTAN, strings.TrimSpace(string(header))):
		l1, err := readLine(r)
		if err != nil {
			return buf, UNKNOWN, "", err
		}
		buf = append(buf, l1...)
		l2, err := readLine(r)
		if err != nil {
			return buf, UNKNOWN, "", err
		}
		buf = append(buf, l2...)
		host := strings.TrimSpace(strings.Split(string(l2), ":")[1])
		return buf, HTTP, host, nil
	default:
		return buf, NORMALTCP, "", nil
	}
}

func readLine(r io.Reader) (buf []byte, err error) {
	b := make([]byte, 1)
	for {
		if _, err := r.Read(b); err != nil {
			return buf, err
		} else {
			buf = append(buf, b[0])
			if b[0] == byte('\n') {
				return buf, nil
			}
		}
	}
}
