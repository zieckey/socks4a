package main

import (
	"bytes"
	"log"
	"net"
)

const bindAddr = ":2011"
const defaultBufSize = 65536
const maxConn = 0x10000

type Tunnel struct {
	lconn  net.Conn // the connection from local app
	rconn  net.Conn // the connection to remote server
	closed bool
}

func refuse(t *Tunnel) {
	buf := []byte{0, 0x5b, 0, 0, 0, 0, 0, 0}
	t.lconn.Write(buf)
	t.closed = true
}

//func grant(t *Tunnel, ip [4]byte, port int16) {
func grant(t *Tunnel) {
	//TODO socks4a need return ip/port
	buf := []byte{0, 0x5a, 0, 0, 0, 0, 0, 0}
	t.lconn.Write(buf)
}

func socks4a(ipBuf []byte) bool {
	if ipBuf[0] == 0 &&
		ipBuf[1] == 0 &&
		ipBuf[2] == 0 &&
		ipBuf[3] != 0 {
		return true
	}

	return false
}

func servRemoteTunnel(t *Tunnel) {
	defer t.rconn.Close()
	buf := make([]byte, defaultBufSize)
	for !t.closed {
		n, err := t.rconn.Read(buf)
		log.Printf("read %v bytes from remote server %v", n, t.rconn.RemoteAddr())
		if n == 0 {
			t.closed = true
			return
		}

		if err != nil {
			log.Printf("Read from remote server %v", err)
			return
		}

		t.lconn.Write(buf[:n])
	}
}

func connectRemote(buf []byte, bufSize int, t *Tunnel) bool {
	// connecting to remote server
	end := bytes.IndexByte(buf[8:], byte(0))
	if end < -1 {
		log.Printf("cannot find '\\0', need to read more data")
		return false // need to read more data
	}
	ver := buf[0]
	cmd := buf[1]
	port := int(buf[2])<<8 + int(buf[3])
	ip := net.IPv4(buf[4], buf[5], buf[6], buf[7])
	log.Printf("ver=%v cmd=%v remote addr %v:%v\n", ver, cmd, ip, port)
	if socks4a(buf[4:]) {
		// TODO get the remote ip
	}

	a := &net.TCPAddr{IP: ip, Port: port}
	c, err := net.DialTCP("tcp", nil, a)
	if err != nil {
		log.Printf("DialTCP", err)
		refuse(t)
		return false
	}
	log.Printf("has connected to remote %s", a)
	t.rconn = c
	if end < bufSize {
		t.rconn.Write(buf[end:])
	}
	go servRemoteTunnel(t)
	return true
}

func servLocalTunnel(lconn net.Conn) {
	t := new(Tunnel)
	t.lconn = lconn
	defer t.lconn.Close()
	buf := make([]byte, defaultBufSize)
	bufSize := 0
	for !t.closed {
		n, _ := t.lconn.Read(buf[bufSize:])
		log.Printf("read %v bytes from local app %v bytes", n, t.lconn.RemoteAddr())
		if n == 0 {
			t.closed = true
			return
		}

		if t.rconn != nil {
			log.Printf("write %v bytes to remote server %v", n, t.rconn.RemoteAddr())
			t.rconn.Write(buf[:n+bufSize])
			bufSize = 0
			continue
		}

		if n+bufSize <= 8 {
			log.Printf("need to recv more data")
			continue
		}

		if connectRemote(buf, bufSize, t) {
			bufSize = 0
			grant(t)
		}
	}
}

func start(addr string) {
	a, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	listener, err := net.ListenTCP("tcp", a)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("socks server bind %v", a)
	for {
		c, err := listener.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		log.Printf("a connection came from %v", c.RemoteAddr().String())
		go servLocalTunnel(c)
	}
}

func main() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Ltime)
	start(bindAddr)
}
