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
	"github.com/sourcegraph/conc"

	"github.com/isobit/ndog/internal"
	"github.com/isobit/ndog/internal/log"
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
	ListenOptionHelp:  listenOptionHelp,
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

type ListenOptions struct {
	MessageType int
}

var listenOptionHelp = ndog.OptionsHelp{}.
	Add("text", "", "Send using text data frames instead of binary")

func extractListenOptions(opts ndog.Options) (ConnectOptions, error) {
	o := ConnectOptions{
		MessageType: websocket.BinaryMessage,
	}

	if _, ok := opts.Pop("text"); ok {
		o.MessageType = websocket.TextMessage
	}

	return o, opts.Done()
}
func Listen(cfg ndog.ListenConfig) error {
	if cfg.URL.Scheme == "wss" {
		return fmt.Errorf("listen does not support secure websockets yet")
	}

	opts, err := extractListenOptions(cfg.Options)
	if err != nil {
		return err
	}

	s := &http.Server{
		Addr: cfg.URL.Host,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Logf(0, "request: %s: %s %s", r.RemoteAddr, r.Method, r.URL)
			if r.Host != cfg.URL.Host {
				log.Logf(1, "request header: Host: %s", r.Host)
			}
			for key, values := range r.Header {
				log.Logf(1, "request header: %s: %s", key, strings.Join(values, ", "))
			}

			upgrader := &websocket.Upgrader{}
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				log.Logf(-1, "error upgrading request: %w", err)
				return
			}
			defer conn.Close()

			log.Logf(2, "upgraded %s", r.RemoteAddr)
			defer log.Logf(1, "closed: %s", r.RemoteAddr)

			stream := cfg.StreamManager.NewStream(r.RemoteAddr)
			defer stream.Close()

			bidirectionalCopy(conn, stream, opts.MessageType)
		}),
	}
	log.Logf(0, "listening: %s", s.Addr)
	return s.ListenAndServe()
}

type ConnectOptions struct {
	Origin      string
	Protocol    string
	Headers     map[string]string
	MessageType int
}

var connectOptionHelp = ndog.OptionsHelp{}.
	Add("header.<NAME>", "<VALUE>", "extra request headers to send").
	Add("origin", "<ORIGIN>", "").
	Add("protocol", "<PROTOCOL>", "").
	Add("text", "", "Send using text data frames instead of binary")

func extractConnectOptions(opts ndog.Options) (ConnectOptions, error) {
	o := ConnectOptions{
		Origin:      "http://localhost",
		Headers:     map[string]string{},
		MessageType: websocket.BinaryMessage,
	}

	if val, ok := opts.Pop("origin"); ok {
		o.Origin = val
	}

	if val, ok := opts.Pop("protocol"); ok {
		o.Protocol = val
	}

	if _, ok := opts.Pop("text"); ok {
		o.MessageType = websocket.TextMessage
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
	log.Logf(0, "connected: %s", remoteAddr)
	defer func() {
		conn.Close()
		log.Logf(0, "closed: %s", remoteAddr)
	}()

	bidirectionalCopy(conn, cfg.Stream, opts.MessageType)

	return nil
}

func bidirectionalCopy(conn *websocket.Conn, stream ndog.Stream, sendMsgType int) {
	wg := conc.WaitGroup{}
	wg.Go(func() {
		defer conn.Close()
		defer stream.Close()

		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					log.Logf(-1, "read error: %s", err)
				}
				return
			}
			log.Logf(2, "received message (type=%d)", msgType)
			if _, err := stream.Writer.Write(append(msg, '\n')); err != nil {
				if !ndog.IsIOClosedErr(err) {
					log.Logf(-1, "read error: %s", err)
				}
				return
			}
		}
	})
	wg.Go(func() {
		defer conn.Close()
		defer stream.Close()

		s := bufio.NewScanner(stream.Reader)
		for s.Scan() {
			if err := conn.WriteMessage(sendMsgType, s.Bytes()); err != nil {
				if !errors.Is(err, net.ErrClosed) {
					log.Logf(-1, "write error: %s", err)
				}
				return
			}
			log.Logf(2, "sent message")
		}
		if err := s.Err(); err != nil {
			if !ndog.IsIOClosedErr(err) {
				log.Logf(-1, "write error: %s", err)
			}
		}
	})
	wg.Wait()
}
