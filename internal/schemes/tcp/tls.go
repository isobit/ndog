package tcp

import (
	"crypto/tls"
	"net"

	"github.com/isobit/ndog/internal"
	"github.com/isobit/ndog/internal/log"
)

var TLSScheme = &ndog.Scheme{
	Names:   []string{"tls"},
	Connect: TLSConnect,
	Listen:  TLSListen,

	Description: `
Connect opens a TLS connection to the server host and port specified in the URL.

Listen starts a TLS server on the host and port specified in the URL.

Examples:
	Echo server: ndog -l 'tls://localhost:8080' -x 'cat'
	`,
}

func TLSListen(cfg ndog.ListenConfig) error {
	tlsConfig, err := cfg.TLS.Config(true, []string{cfg.URL.Hostname()})
	if err != nil {
		return err
	}

	tcpListener, err := cfg.Net.Listen("tcp", cfg.URL.Host)
	if err != nil {
		return err
	}
	listener := tls.NewListener(tcpListener, tlsConfig)
	defer listener.Close()
	log.Logf(0, "listening: %s", listener.Addr())

	handleConn := func(conn net.Conn) {
		defer conn.Close()

		remoteAddr := conn.RemoteAddr()
		log.Logf(1, "accepted: %s", remoteAddr)
		defer log.Logf(1, "closed: %s", remoteAddr)

		stream := cfg.StreamManager.NewStream(remoteAddr.String())
		defer stream.Close()

		bidirectionalCopy(conn, stream)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			if conn != nil {
				log.Logf(-1, "accept error: %s: %s", conn.RemoteAddr(), err)
			} else {
				log.Logf(-1, "accept error: %s", err)
			}
			continue
		}
		go handleConn(conn)
	}
}

func TLSConnect(cfg ndog.ConnectConfig) error {
	tlsConfig, err := cfg.TLS.Config(false, nil)
	if err != nil {
		return err
	}

	conn, err := tls.Dial("tcp", cfg.URL.Host, tlsConfig)
	if err != nil {
		return err
	}
	defer conn.Close()

	remoteAddr := conn.RemoteAddr()
	log.Logf(0, "connected: %s", remoteAddr)
	defer log.Logf(0, "closed: %s", remoteAddr)

	bidirectionalCopy(conn, cfg.Stream)

	return nil
}
