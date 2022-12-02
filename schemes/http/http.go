package http

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/isobit/ndog"
)

var Scheme = &ndog.Scheme{
	Names:   []string{"http"},
	Connect: nil,
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
			io.Copy(cfg.Out, r.Body)
			fmt.Fprintln(cfg.Out)
		}),
	}
	cfg.Logf("listening: %s", s.Addr)
	return s.ListenAndServe()
}
