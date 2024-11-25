package udp

import (
	"bufio"
	"fmt"
	"io"
	"net"

	"github.com/isobit/ndog/internal"
	"github.com/isobit/ndog/internal/ioutil"
	"github.com/isobit/ndog/internal/log"
)

var Scheme = &ndog.Scheme{
	Names:            []string{"udp"},
	Connect:          Connect,
	Listen:           Listen,
	ListenOptionHelp: listenOptionHelp,

	Description: `
Connect opens a UDP connection to the server host and port specified in the URL.

Listen starts a UDP server on the host and port specified in the URL.

Examples:
	Echo server: ndog -l 'udp://localhost:8080' -x 'cat'
	`,
}

type listenOptions struct {
	Lines bool
	JSON  bool
}

var listenOptionHelp = ndog.OptionsHelp{}.
	Add("lines", "", "delimit individual packets with line breaks").
	Add("json", "", "output JSON representation of incoming packets")

func extractListenOptions(opts ndog.Options) (listenOptions, error) {
	o := listenOptions{}
	if _, ok := opts.Pop("json"); ok {
		o.JSON = true
	}
	if _, ok := opts.Pop("lines"); ok {
		o.Lines = true
	}
	return o, opts.Done()
}

func Listen(cfg ndog.ListenConfig) error {
	opts, err := extractListenOptions(cfg.Options)
	if err != nil {
		return err
	}

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
		data := buf[:nr]
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
			if opts.Lines {
				go func() {
					scanner := bufio.NewScanner(stream.Reader)
					for scanner.Scan() {
						// TODO handle errors
						conn.WriteTo(scanner.Bytes(), remoteAddr)
					}
				}()
			} else {
				go io.Copy(&connWriter{conn: conn, addr: remoteAddr}, stream.Reader)
			}
		}

		if opts.JSON {
			if err := ioutil.WriteJSON(stream.Writer, struct {
				RemoteAddr string
				Data       []byte
			}{
				RemoteAddr: remoteAddrStr,
				Data:       data,
			}); err != nil {
				return err
			}
		} else {
			if _, err := stream.Writer.Write(data); err != nil {
				return err
			}
			if opts.Lines {
				if _, err := stream.Writer.Write([]byte{'\n'}); err != nil {
					return err
				}
			}
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
