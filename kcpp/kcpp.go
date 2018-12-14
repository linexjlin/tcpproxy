package kcpp

import (
	"crypto/sha1"
	"io"
	"math/rand"
	"net"
	_ "net/http/pprof"
	"time"

	"github.com/golang/snappy"
	"github.com/linexjlin/simple-log"
	kcp "github.com/xtaci/kcp-go"
	"github.com/xtaci/smux"
	"golang.org/x/crypto/pbkdf2"
)

var (
	// VERSION is injected by buildflags
	VERSION = "SELFBUILD"
	// SALT is use for pbkdf2 key expansion
	SALT = "kcp-go"
)

type compStream struct {
	conn net.Conn
	w    *snappy.Writer
	r    *snappy.Reader
}

func (c *compStream) Read(p []byte) (n int, err error) {
	return c.r.Read(p)
}

func (c *compStream) Write(p []byte) (n int, err error) {
	n, err = c.w.Write(p)
	err = c.w.Flush()
	return n, err
}

func (c *compStream) Close() error {
	return c.conn.Close()
}

func newCompStream(conn net.Conn) *compStream {
	c := new(compStream)
	c.conn = conn
	c.w = snappy.NewBufferedWriter(conn)
	c.r = snappy.NewReader(conn)
	return c
}

// handle multiplex-ed connection
func handleMux(conn io.ReadWriteCloser, config *Config, raddr net.Addr, forward func(io.ReadWriteCloser, net.Addr)) {
	// stream multiplex
	smuxConfig := smux.DefaultConfig()
	smuxConfig.MaxReceiveBuffer = config.SockBuf
	smuxConfig.KeepAliveInterval = time.Duration(config.KeepAlive) * time.Second

	mux, err := smux.Server(conn, smuxConfig)
	if err != nil {
		log.Println(err)
		return
	}
	defer mux.Close()
	for {
		p1, err := mux.AcceptStream()
		if err != nil {
			log.Warning(err)
			return
		} else {
			if p1 != nil {
				go forward(p1, raddr)
			}
		}
		//go handleClient(p1, p2, config.Quiet)
	}
}

func init() {
	go sigHandler()
	rand.Seed(int64(time.Now().Nanosecond()))
	/*if VERSION == "SELFBUILD" {
		// add more log flags for debugging
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}*/

}

func ListenKCP(listenAddr string, forward func(io.ReadWriteCloser, net.Addr)) {
	var config Config

	//config.Listen = ":80"
	//config.Target = "127.0.0.1:80"
	config.Key = "it's a secrect"
	config.Crypt = "none"
	config.Mode = "fast"
	config.MTU = 1350
	config.SndWnd = 1024
	config.RcvWnd = 1024
	config.DataShard = 10
	config.ParityShard = 3
	config.DSCP = 0
	config.NoComp = false
	config.AckNodelay = false
	config.NoDelay = 0
	config.Interval = 0
	config.Resend = 0
	config.NoCongestion = 0
	config.SockBuf = 4194304
	config.KeepAlive = 10
	config.Log = ""
	config.SnmpLog = ""
	config.SnmpPeriod = 60
	config.Pprof = false
	config.Quiet = false

	switch config.Mode {
	case "normal":
		config.NoDelay, config.Interval, config.Resend, config.NoCongestion = 0, 40, 2, 1
	case "fast":
		config.NoDelay, config.Interval, config.Resend, config.NoCongestion = 0, 30, 2, 1
	case "fast2":
		config.NoDelay, config.Interval, config.Resend, config.NoCongestion = 1, 20, 2, 1
	case "fast3":
		config.NoDelay, config.Interval, config.Resend, config.NoCongestion = 1, 10, 2, 1
	}

	log.Println("version:", VERSION)
	log.Println("initiating key derivation")
	pass := pbkdf2.Key([]byte(config.Key), []byte(SALT), 4096, 32, sha1.New)
	var block kcp.BlockCrypt
	switch config.Crypt {
	case "sm4":
		block, _ = kcp.NewSM4BlockCrypt(pass[:16])
	case "tea":
		block, _ = kcp.NewTEABlockCrypt(pass[:16])
	case "xor":
		block, _ = kcp.NewSimpleXORBlockCrypt(pass)
	case "none":
		block, _ = kcp.NewNoneBlockCrypt(pass)
	case "aes-128":
		block, _ = kcp.NewAESBlockCrypt(pass[:16])
	case "aes-192":
		block, _ = kcp.NewAESBlockCrypt(pass[:24])
	case "blowfish":
		block, _ = kcp.NewBlowfishBlockCrypt(pass)
	case "twofish":
		block, _ = kcp.NewTwofishBlockCrypt(pass)
	case "cast5":
		block, _ = kcp.NewCast5BlockCrypt(pass[:16])
	case "3des":
		block, _ = kcp.NewTripleDESBlockCrypt(pass[:24])
	case "xtea":
		block, _ = kcp.NewXTEABlockCrypt(pass[:16])
	case "salsa20":
		block, _ = kcp.NewSalsa20BlockCrypt(pass)
	default:
		config.Crypt = "aes"
		block, _ = kcp.NewAESBlockCrypt(pass)
	}
	log.Info("listen on UDP:", listenAddr)
	lis, err := kcp.ListenWithOptions(listenAddr, block, config.DataShard, config.ParityShard)
	if err != nil {
		log.Warning(err)
		return
	}
	//log.Println("listening on:", lis.Addr())
	//log.Println("target:", config.Target)
	log.Println("encryption:", config.Crypt)
	log.Println("nodelay parameters:", config.NoDelay, config.Interval, config.Resend, config.NoCongestion)
	log.Println("sndwnd:", config.SndWnd, "rcvwnd:", config.RcvWnd)
	log.Println("compression:", !config.NoComp)
	log.Println("mtu:", config.MTU)
	log.Println("datashard:", config.DataShard, "parityshard:", config.ParityShard)
	log.Println("acknodelay:", config.AckNodelay)
	log.Println("dscp:", config.DSCP)
	log.Println("sockbuf:", config.SockBuf)
	log.Println("keepalive:", config.KeepAlive)
	log.Println("snmplog:", config.SnmpLog)
	log.Println("snmpperiod:", config.SnmpPeriod)
	log.Println("pprof:", config.Pprof)
	log.Println("quiet:", config.Quiet)

	if err := lis.SetDSCP(config.DSCP); err != nil {
		log.Warning("SetDSCP:", err)
	}
	if err := lis.SetReadBuffer(config.SockBuf); err != nil {
		log.Warning("SetReadBuffer:", err)
	}
	if err := lis.SetWriteBuffer(config.SockBuf); err != nil {
		log.Warning("SetWriteBuffer:", err)
	}

	for {
		if conn, err := lis.AcceptKCP(); err == nil {
			log.Println("remote address:", conn.RemoteAddr())
			conn.SetStreamMode(true)
			conn.SetWriteDelay(false)
			conn.SetNoDelay(config.NoDelay, config.Interval, config.Resend, config.NoCongestion)
			conn.SetMtu(config.MTU)
			conn.SetWindowSize(config.SndWnd, config.RcvWnd)
			conn.SetACKNoDelay(config.AckNodelay)

			if config.NoComp {
				go handleMux(conn, &config, conn.RemoteAddr(), forward)
			} else {
				go handleMux(newCompStream(conn), &config, conn.RemoteAddr(), forward)
			}
		} else {
			log.Warning("%+v", err)
		}
	}
}
