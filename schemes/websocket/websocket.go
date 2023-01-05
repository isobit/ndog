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

func Listen(cfg ndog.ListenConfig) error {
	s := &http.Server{
		Addr: cfg.URL.Host,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ndog.Logf(1, "request: %s: %s %s", r.RemoteAddr, r.Method, r.URL)
			for key, values := range r.Header {
				ndog.Logf(2, "request header: %s: %s", key, strings.Join(values, ", "))
			}
			wsHandler := websocket.Handler(func(conn *websocket.Conn) {
				stream := cfg.StreamFactory.NewStream(r.RemoteAddr)
				defer stream.Close()

				go io.Copy(conn, stream)

				buf := make([]byte, 1024)
				for {
					nr, err := conn.Read(buf)
					if err != nil {
						return
					}
					ndog.Logf(2, "read: %d bytes from %s", nr, r.RemoteAddr)

					_, err = stream.Write(buf[:nr])
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

func Connect(cfg ndog.ConnectConfig) error {
	conn, err := websocket.Dial(cfg.URL.String(), "", "http://localhost")
	if err != nil {
		return err
	}
	defer conn.Close()

	remoteAddr := conn.RemoteAddr()
	ndog.Logf(0, "connected: %s", remoteAddr)

	stream := cfg.Stream

	go io.Copy(conn, stream)
	_, err = io.Copy(stream, conn)

	ndog.Logf(0, "closed: %s", remoteAddr)
	return err
}
