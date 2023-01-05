package tcp

import (
	"fmt"
	"io"
	"net"

	"github.com/isobit/ndog"
)

var Scheme = &ndog.Scheme{
	Names:   []string{"tcp"},
	Connect: Connect,
	Listen:  Listen,
}

func Listen(cfg ndog.ListenConfig) error {
	addr, err := net.ResolveTCPAddr("tcp", cfg.URL.Host)
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	ndog.Logf(0, "listening: %s", listener.Addr())

	handleConn := func(conn *net.TCPConn) {
		defer conn.Close()

		remoteAddr := conn.RemoteAddr()

		stream := cfg.StreamFactory.NewStream(remoteAddr.String())
		defer stream.Close()
		go io.Copy(conn, stream)

		buf := make([]byte, 1024)
		for {
			nr, err := conn.Read(buf)
			if err != nil {
				return
			}
			_, err = stream.Write(buf[:nr])
			if err != nil {
				return
			}
		}
	}

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			if conn != nil {
				ndog.Logf(-1, "accept error: %s: %s", conn.RemoteAddr(), err)
			} else {
				ndog.Logf(-1, "accept error: %s", err)
			}
			continue
		}
		ndog.Logf(1, "accepted: %s", conn.RemoteAddr())
		go func() {
			handleConn(conn)
			ndog.Logf(1, "closed: %s", conn.RemoteAddr())
		}()
	}
}

func Connect(cfg ndog.ConnectConfig) error {
	addr, err := net.ResolveTCPAddr("tcp", cfg.URL.Host)
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	remoteAddr := conn.RemoteAddr()
	ndog.Logf(0, "connected: %s", remoteAddr)

	// stream := cfg.NewStream(remoteAddr.String())
	// defer stream.Close()
	stream := cfg.Stream

	go io.Copy(conn, stream)
	_, err = io.Copy(stream, conn)

	ndog.Logf(0, "closed: %s", remoteAddr)
	return err
}

type Request struct {
	RemoteAddr string
	Data       []byte
}

type Response struct {
	Data []byte
}
