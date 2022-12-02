package udp

import (
	"fmt"
	"io"
	"net"

	"github.com/isobit/ndog"
)

var Scheme = &ndog.Scheme{
	Names:   []string{"udp"},
	Connect: Connect,
	Listen:  Listen,
}

func Listen(cfg ndog.Config) error {
	addr, err := net.ResolveUDPAddr("udp", cfg.URL.Host)
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	cfg.Logf("listening: %s", conn.LocalAddr())

	// _, err = io.Copy(cfg.Out, conn)
	// return err

	buf := make([]byte, 1024)
	for {
		nr, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if err == io.EOF || err == net.ErrClosed {
				return nil
			}
			return err
		}
		if cfg.Verbose {
			cfg.Logf("read: %d bytes from %s", nr, remoteAddr)
		}

		_, err = cfg.Out.Write(buf[:nr])
		if err != nil {
			return err
		}
	}
}

func Connect(cfg ndog.Config) error {
	addr, err := net.ResolveUDPAddr("udp", cfg.URL.Host)
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	cfg.Logf("connected: %s", conn.RemoteAddr())

	go func() {
		io.Copy(conn, cfg.In)
	}()

	_, err = io.Copy(cfg.Out, conn)
	return err
}
