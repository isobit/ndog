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

func Listen(cfg ndog.ListenConfig) error {
	addr, err := net.ResolveUDPAddr("udp", cfg.URL.Host)
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	ndog.Logf(0, "listening: %s", conn.LocalAddr())

	streams := map[string]ndog.Stream{}

	buf := make([]byte, 1024)
	for {
		nr, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if err == io.EOF || err == net.ErrClosed {
				return nil
			}
			return err
		}
		remoteAddrStr := remoteAddr.String()

		var stream ndog.Stream
		if existingStream, ok := streams[remoteAddrStr]; ok {
			ndog.Logf(10, "using existing stream: %s", remoteAddrStr)
			stream = existingStream
		} else {
			ndog.Logf(10, "creating new stream: %s", remoteAddrStr)
			stream = cfg.StreamManager.NewStream(remoteAddrStr)
			// TODO close stream reader on timeout
			streams[remoteAddrStr] = stream
			go io.Copy(newUDPWriter(conn, remoteAddr), stream.Reader)
		}

		// TODO close stream writer on timeout
		_, err = stream.Writer.Write(buf[:nr])
		if err != nil {
			return err
		}
	}
}

type udpWriter struct {
	conn *net.UDPConn
	addr *net.UDPAddr
}

func newUDPWriter(conn *net.UDPConn, addr *net.UDPAddr) *udpWriter {
	return &udpWriter{
		conn: conn,
		addr: addr,
	}
}

func (uw *udpWriter) Write(p []byte) (int, error) {
	return uw.conn.WriteToUDP(p, uw.addr)
}

func Connect(cfg ndog.ConnectConfig) error {
	addr, err := net.ResolveUDPAddr("udp", cfg.URL.Host)
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	remoteAddr := conn.RemoteAddr()
	ndog.Logf(0, "connected: %s", remoteAddr)

	stream := cfg.Stream

	go io.Copy(conn, stream.Reader)
	_, err = io.Copy(stream.Writer, conn)

	ndog.Logf(0, "closed: %s", remoteAddr)
	return err
}
