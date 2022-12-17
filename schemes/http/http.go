package http

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/tinylib/msgp/msgp"

	"github.com/isobit/ndog"
)

var HTTPScheme = &ndog.Scheme{
	Names:   []string{"http"},
	Connect: Connect,
	Listen:  Listen,
}

var HTTPSScheme = &ndog.Scheme{
	Names:   []string{"https"},
	Connect: Connect,
}

type Options struct {
	StatusCode       int
	Headers          map[string]string
	FixedResponse    string
	ServeFile        string
	WriteRequestLine bool
	MsgpackToJSON    bool
}

func ExtractOptions(cfg ndog.Config) (Options, error) {
	opts := Options{
		StatusCode: 200,
		Headers:    map[string]string{},
	}
	if _, ok := cfg.PopOption("request_line"); ok {
		opts.WriteRequestLine = true
	}
	if _, ok := cfg.PopOption("msgpack_to_json"); ok {
		opts.MsgpackToJSON = true
	}

	headerKeyPrefix := "header."
	for key, val := range cfg.Options {
		if !strings.HasPrefix(key, headerKeyPrefix) {
			continue
		}
		headerKey := strings.TrimPrefix(key, headerKeyPrefix)
		opts.Headers[headerKey] = val
		delete(cfg.Options, key)
	}

	if val, ok := cfg.Options["status_code"]; ok {
		if _, err := fmt.Sscanf(val, "%d", &opts.StatusCode); err != nil {
			return opts, fmt.Errorf("error parsing status_code option: %w", err)
		}
	}

	if serveFilePath, ok := cfg.PopOption("serve_file"); ok {
		serveFileAbsPath, err := filepath.Abs(serveFilePath)
		if err != nil {
			return opts, fmt.Errorf("error parsing serve_file option: %w", err)
		}
		opts.ServeFile = serveFileAbsPath
	}

	if val, ok := cfg.PopOption("fixed_response"); ok {
		opts.FixedResponse = val
	}

	return opts, cfg.CheckRemainingOptions()
}

func Listen(cfg ndog.Config) error {
	opts, err := ExtractOptions(cfg)
	if err != nil {
		return err
	}
	if opts.ServeFile != "" {
		ndog.Logf(1, "http: will serve file(s) from %s", opts.ServeFile)
	}

	s := &http.Server{
		Addr: cfg.URL.Host,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ndog.Logf(1, "request: %s: %s %s", r.RemoteAddr, r.Method, r.URL)
			for key, values := range r.Header {
				ndog.Logf(2, "request header: %s: %s", key, strings.Join(values, ", "))
			}

			stream := cfg.NewStream(r.RemoteAddr)
			defer stream.Close()

			if opts.WriteRequestLine {
				fmt.Fprintf(stream, "%s %s %s\n", r.Method, r.URL, r.Proto)
			}

			// Receive request.
			contentType := r.Header.Get("Content-Type")
			if opts.MsgpackToJSON && (contentType == "application/msgpack" || contentType == "application/x-msgpack") {
				body, err := ioutil.ReadAll(r.Body)
				if err != nil {
					ndog.Logf(-1, "error reading request body: %s", err)
					return
				}
				if _, err := msgp.UnmarshalAsJSON(stream, body); err != nil {
					ndog.Logf(-1, "error unmarshaling request body msgpack as JSON: %s", err)
					return
				}
				io.WriteString(stream, "\n")
			} else {
				io.Copy(stream, r.Body)
			}
			stream.CloseWriter()

			// Send response.
			if opts.ServeFile != "" {
				http.ServeFile(w, r, filepath.Join(opts.ServeFile, r.URL.Path))
			} else {
				for key, val := range opts.Headers {
					w.Header().Add(key, val)
				}
				w.WriteHeader(opts.StatusCode)
				if opts.FixedResponse != "" {
					io.WriteString(w, opts.FixedResponse)
				} else {
					io.Copy(w, stream)
				}
			}
		}),
	}
	ndog.Logf(0, "listening: %s", s.Addr)
	return s.ListenAndServe()
}

func Connect(cfg ndog.Config) error {
	ndog.Logf(0, "request: GET %s", cfg.URL.RequestURI())
	stream := cfg.NewStream("")
	defer stream.Close()
	resp, err := http.Get(cfg.URL.String())
	if err != nil {
		return err
	}
	ndog.Logf(0, "response: %s", resp.Status)
	for key, values := range resp.Header {
		ndog.Logf(1, "response header: %s: %s", key, strings.Join(values, ", "))
	}
	io.Copy(stream, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("got error response: %s", resp.Status)
	}
	return nil
}
