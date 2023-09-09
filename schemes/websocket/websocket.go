package websocket

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/net/websocket"

	"github.com/isobit/ndog"
)

var Scheme = &ndog.Scheme{
	Names:   []string{"ws", "wss"},
	Listen:  Listen,
	Connect: Connect,
}

func Listen(cfg ndog.ListenConfig) error {
	if cfg.URL.Scheme != "ws" {
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
			wsHandler := websocket.Handler(func(conn *websocket.Conn) {
				stream := cfg.StreamManager.NewStream(r.RemoteAddr)
				defer stream.Close()

				go io.Copy(conn, stream.Reader)

				buf := make([]byte, 1024)
				for {
					nr, err := conn.Read(buf)
					if err != nil {
						return
					}
					ndog.Logf(2, "read: %d bytes from %s", nr, r.RemoteAddr)

					_, err = stream.Writer.Write(buf[:nr])
					if err != nil {
						return
					}
				}
			})
			wsHandler.ServeHTTP(w, r)
			ndog.Logf(1, "closed: %s", r.RemoteAddr)
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

	wsCfg, err := websocket.NewConfig(cfg.URL.String(), opts.Origin)
	if err != nil {
		return err
	}
	if opts.Protocol != "" {
		wsCfg.Protocol = []string{opts.Protocol}
	}
	for key, val := range opts.Headers {
		wsCfg.Header.Add(key, val)
	}

	conn, err := websocket.DialConfig(wsCfg)
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
