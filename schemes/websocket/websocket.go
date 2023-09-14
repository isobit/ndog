package websocket

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/isobit/ndog"
)

var WSScheme = &ndog.Scheme{
	Names:   []string{"ws"},
	Listen:  Listen,
	Connect: Connect,

	Description: `
Connect opens a WebSocket connection to the specified URL.

Listen starts a WebSocket server on the host and port specified in the URL.

Examples:
	Echo server: ndog -l 'ws://localhost:8080' -x 'cat'
	`,
	ConnectOptionHelp: connectOptionHelp,
}

var WSSScheme = &ndog.Scheme{
	Names:   []string{"wss"},
	Connect: Connect,

	Description: `
Connect opens a WebSocket connection to the specified URL.
	`,
	ConnectOptionHelp: connectOptionHelp,
}

func Listen(cfg ndog.ListenConfig) error {
	if cfg.URL.Scheme == "wss" {
		return fmt.Errorf("listen does not support secure websockets yet")
	}
	s := &http.Server{
		Addr: cfg.URL.Host,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ndog.Logf(0, "request: %s: %s %s", r.RemoteAddr, r.Method, r.URL)
			if r.Host != cfg.URL.Host {
				ndog.Logf(1, "request header: Host: %s", r.Host)
			}
			for key, values := range r.Header {
				ndog.Logf(1, "request header: %s: %s", key, strings.Join(values, ", "))
			}
			upgrader := &websocket.Upgrader{}
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				// TODO
				return
			}
			defer func() {
				conn.Close()
				ndog.Logf(1, "closed: %s", r.RemoteAddr)
			}()
			ndog.Logf(2, "upgraded %s", r.RemoteAddr)

			stream := cfg.StreamManager.NewStream(r.RemoteAddr)
			defer stream.Close()

			go func() {
				for {
					msgType, msg, err := conn.ReadMessage()
					if err != nil {
						ndog.Logf(-1, "read message error: %s", err)
						if errors.Is(err, net.ErrClosed) {
							stream.Close()
						}
						return
					}
					ndog.Logf(2, "received message: %v", msgType)
					if _, err := stream.Writer.Write(msg); err != nil {
						ndog.Logf(-1, "write message error: %s", err)
						return
					}
					fmt.Fprintln(stream.Writer)
				}

			}()

			s := bufio.NewScanner(stream.Reader)
			for s.Scan() {
				if err := conn.WriteMessage(websocket.BinaryMessage, s.Bytes()); err != nil {
					ndog.Logf(-1, "write message error: %s", err)
					if errors.Is(err, net.ErrClosed) {
						stream.Close()
					}
					return
				}
				ndog.Logf(2, "sent message")
			}
			if err := s.Err(); err != nil {
				ndog.Logf(2, "scan err: %s", err)
			}
		}),
	}
	ndog.Logf(0, "listening: %s", s.Addr)
	return s.ListenAndServe()
}

type ConnectOptions struct {
	Origin   string
	Protocol string
	Headers  map[string]string
}

var connectOptionHelp = ndog.OptionsHelp{}.
	Add("header.<NAME>", "<VALUE>", "extra request headers to send").
	Add("origin", "<ORIGIN>", "").
	Add("protocol", "<PROTOCOL>", "")

func extractConnectOptions(opts ndog.Options) (ConnectOptions, error) {
	o := ConnectOptions{
		Origin:  "http://localhost",
		Headers: map[string]string{},
	}

	if val, ok := opts.Pop("origin"); ok {
		o.Origin = val
	}

	if val, ok := opts.Pop("protocol"); ok {
		o.Protocol = val
	}

	headerKeyPrefix := "header."
	for key, val := range opts {
		if !strings.HasPrefix(key, headerKeyPrefix) {
			continue
		}
		headerKey := strings.TrimPrefix(key, headerKeyPrefix)
		o.Headers[headerKey] = val
		delete(opts, key)
	}

	return o, opts.Done()
}

func Connect(cfg ndog.ConnectConfig) error {
	opts, err := extractConnectOptions(cfg.Options)
	if err != nil {
		return err
	}

	header := http.Header{}
	for key, val := range opts.Headers {
		header.Add(key, val)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	if opts.Protocol != "" {
		dialer.Subprotocols = []string{opts.Protocol}
	}

	conn, _, err := dialer.Dial(cfg.URL.String(), header)
	if err != nil {
		return err
	}

	remoteAddr := conn.RemoteAddr()
	ndog.Logf(0, "connected: %s", remoteAddr)
	defer func() {
		conn.Close()
		ndog.Logf(0, "closed: %s", remoteAddr)
	}()

	stream := cfg.Stream

	go func() {
		defer stream.Writer.Close()
		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				ndog.Logf(-1, "read message error: %s", err)
				if errors.Is(err, net.ErrClosed) {
					stream.Close()
				}
				return
			}
			ndog.Logf(2, "received message: %v", msgType)
			if _, err := stream.Writer.Write(msg); err != nil {
				ndog.Logf(-1, "write message error: %s", err)
				return
			}
			fmt.Fprintln(stream.Writer)
		}
	}()

	defer stream.Reader.Close()
	s := bufio.NewScanner(stream.Reader)
	for s.Scan() {
		if err := conn.WriteMessage(websocket.BinaryMessage, s.Bytes()); err != nil {
			if errors.Is(err, net.ErrClosed) {
				stream.Close()
			}
			return err
		}
	}
	return s.Err()
}
