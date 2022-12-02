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

func Listen(cfg ndog.Config) error {
	addr, err := net.ResolveTCPAddr("tcp", cfg.URL.Host)
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	cfg.Logf("listening: %s", listener.Addr())

	in := ndog.NewFanoutLineReader(cfg.In)
	go in.ScanLoop()

	handleConn := func(conn *net.TCPConn) {
		defer conn.Close()

		in, cancel := in.Tee()
		defer cancel()
		go io.Copy(conn, in)

		// io.Copy(out, conn)
		remoteAddr := conn.RemoteAddr()
		buf := make([]byte, 1024)
		for {
			nr, err := conn.Read(buf)
			if err != nil {
				return
			}
			if cfg.Verbose {
				cfg.Logf("read: %d bytes from %s", nr, remoteAddr)
			}
			_, err = cfg.Out.Write(buf[:nr])
			if err != nil {
				return
			}
		}
	}

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			if conn != nil {
				cfg.Logf("accept error: %s: %s", conn.RemoteAddr(), err)
			} else {
				cfg.Logf("accept error: %s", err)
			}
			continue
		}
		cfg.Logf("accepted: %s", conn.RemoteAddr())
		go func() {
			handleConn(conn)
			cfg.Logf("closed: %s", conn.RemoteAddr())
		}()
	}
}

func Connect(cfg ndog.Config) error {
	addr, err := net.ResolveTCPAddr("tcp", cfg.URL.Host)
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	cfg.Logf("connected: %s", conn.RemoteAddr())

	go func() {
		io.Copy(conn, cfg.In)
	}()

	_, err = io.Copy(cfg.Out, conn)
	cfg.Logf("closed: %s", conn.RemoteAddr())
	return err
}
