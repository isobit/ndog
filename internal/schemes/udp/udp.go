package udp

import (
	"fmt"
	"io"
	"net"

	"github.com/isobit/ndog/internal"
	"github.com/isobit/ndog/internal/log"
)

var Scheme = &ndog.Scheme{
	Names:   []string{"udp"},
	Connect: Connect,
	Listen:  Listen,

	Description: `
Connect opens a UDP connection to the server host and port specified in the URL.

Listen starts a UDP server on the host and port specified in the URL.

Examples:
	Echo server: ndog -l 'udp://localhost:8080' -x 'cat'
	`,
}

func Listen(cfg ndog.ListenConfig) error {
	conn, err := cfg.Net.ListenPacket("udp", cfg.URL.Host)
	if err != nil {
		return err
	}
	defer conn.Close()
	log.Logf(0, "listening: %s", conn.LocalAddr())

	streams := map[string]ndog.Stream{}

	// ReadFrom dequeues an entire packet from the socket each time it's
	// called, so the buffer needs to have enough space to read entire packets
	// at once. 65535 bytes is the maximum possible packet size; ndog doesn't
	// know anything about the expected inner protocol, so best to use this
	// maximum size as the buffer size.
	buf := make([]byte, 65535)
	for {
		nr, remoteAddr, err := conn.ReadFrom(buf)
		if err != nil {
			if err == io.EOF || err == net.ErrClosed {
				return nil
			}
			return err
		}
		remoteAddrStr := remoteAddr.String()
		log.Logf(10, "%d bytes from %s", nr, remoteAddrStr)

		var stream ndog.Stream
		if existingStream, ok := streams[remoteAddrStr]; ok {
			log.Logf(10, "using existing stream: %s", remoteAddrStr)
			stream = existingStream
		} else {
			log.Logf(10, "creating new stream: %s", remoteAddrStr)
			stream = cfg.StreamManager.NewStream(remoteAddrStr)
			// TODO close stream reader on timeout
			streams[remoteAddrStr] = stream
			go io.Copy(&connWriter{conn: conn, addr: remoteAddr}, stream.Reader)
		}

		// TODO close stream writer on timeout
		_, err = stream.Writer.Write(buf[:nr])
		if err != nil {
			return err
		}
	}
}

type connWriter struct {
	conn net.PacketConn
	addr net.Addr
}

func (w *connWriter) Write(p []byte) (int, error) {
	return w.conn.WriteTo(p, w.addr)
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
	log.Logf(0, "connected: %s", remoteAddr)

	stream := cfg.Stream

	go io.Copy(conn, stream.Reader)
	_, err = io.Copy(stream.Writer, conn)

	log.Logf(0, "closed: %s", remoteAddr)
	return err
}
