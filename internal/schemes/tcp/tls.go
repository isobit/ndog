package tcp

import (
	"crypto/tls"
	"errors"
	// "fmt"
	"io"
	"net"

	"github.com/sourcegraph/conc"

	"github.com/isobit/ndog/internal"
	"github.com/isobit/ndog/internal/log"
)

var TLSScheme = &ndog.Scheme{
	Names:   []string{"tls"},
	Connect: TLSConnect,
	// Listen:  Listen,

	Description: `
Connect opens a TCP connection to the server host and port specified in the URL.

Listen starts a TCP server on the host and port specified in the URL.

Examples:
	Echo server: ndog -l 'tcp://localhost:8080' -x 'cat'
	`,
}

// func Listen(cfg ndog.ListenConfig) error {
// 	addr, err := net.ResolveTCPAddr("tcp", cfg.URL.Host)
// 	if err != nil {
// 		return fmt.Errorf("invalid address: %w", err)
// 	}

// 	listener, err := net.ListenTCP("tcp", addr)
// 	if err != nil {
// 		return err
// 	}
// 	defer listener.Close()
// 	log.Logf(0, "listening: %s", listener.Addr())

// 	handleConn := func(conn *net.TCPConn) {
// 		defer conn.Close()

// 		remoteAddr := conn.RemoteAddr()
// 		log.Logf(1, "accepted: %s", remoteAddr)
// 		defer log.Logf(1, "closed: %s", remoteAddr)

// 		stream := cfg.StreamManager.NewStream(remoteAddr.String())
// 		defer stream.Close()

// 		bidirectionalCopy(conn, stream)
// 	}

// 	for {
// 		conn, err := listener.AcceptTCP()
// 		if err != nil {
// 			if conn != nil {
// 				log.Logf(-1, "accept error: %s: %s", conn.RemoteAddr(), err)
// 			} else {
// 				log.Logf(-1, "accept error: %s", err)
// 			}
// 			continue
// 		}
// 		go handleConn(conn)
// 	}
// }

func TLSConnect(cfg ndog.ConnectConfig) error {
	// addr, err := net.ResolveTCPAddr("tcp", cfg.URL.Host)
	// if err != nil {
	// 	return fmt.Errorf("invalid address: %w", err)
	// }

	conn, err := tls.Dial("tcp", cfg.URL.Host, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	remoteAddr := conn.RemoteAddr()
	log.Logf(0, "connected: %s", remoteAddr)
	defer log.Logf(0, "closed: %s", remoteAddr)

	tlsBidirectionalCopy(conn, cfg.Stream)

	return nil
}

func tlsBidirectionalCopy(conn *tls.Conn, stream ndog.Stream) {
	wg := conc.WaitGroup{}
	wg.Go(func() {
		defer conn.Close()
		defer stream.Close()

		if _, err := io.Copy(conn, stream.Reader); err != nil {
			if !ndog.IsIOClosedErr(err) && !errors.Is(err, net.ErrClosed) {
				log.Logf(-1, "write error: %s", err)
			}
		}
	})
	wg.Go(func() {
		defer conn.Close()
		defer stream.Close()

		if _, err := io.Copy(stream.Writer, conn); err != nil {
			if !ndog.IsIOClosedErr(err) && !errors.Is(err, net.ErrClosed) {
				log.Logf(-1, "read error: %s", err)
			}
		}
	})
	wg.Wait()
}
