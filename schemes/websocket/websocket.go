package websocket

import (
	"io"
	"net/http"
	"strings"

	"golang.org/x/net/websocket"

	"github.com/isobit/ndog"
)

var Scheme = &ndog.Scheme{
	Names:   []string{"ws"},
	Connect: Connect,
	Listen:  Listen,
}

func Listen(cfg ndog.Config) error {
	s := &http.Server{
		Addr: cfg.URL.Host,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cfg.Logf("request: %s: %s %s", r.RemoteAddr, r.Method, r.URL)
			if cfg.Verbose {
				for key, values := range r.Header {
					cfg.Logf("request header: %s: %s", key, strings.Join(values, ", "))
				}
			}
			wsHandler := websocket.Handler(func(conn *websocket.Conn) {
				buf := make([]byte, 1024)
				for {
					nr, err := conn.Read(buf)
					if err != nil {
						return
					}
					if cfg.Verbose {
						cfg.Logf("read: %d bytes from %s", nr, r.RemoteAddr)
					}

					_, err = cfg.Out.Write(buf[:nr])
					if err != nil {
						return
					}
				}
			})
			wsHandler.ServeHTTP(w, r)
			cfg.Logf("closed: %s", r.RemoteAddr)
		}),
	}
	cfg.Logf("listening: %s", s.Addr)
	return s.ListenAndServe()
}

func Connect(cfg ndog.Config) error {
	conn, err := websocket.Dial(cfg.URL.String(), "", "http://localhost")
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
