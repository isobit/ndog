package http

import (
	"bufio"
	"fmt"
	"io"
	stdlog "log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/tinylib/msgp/msgp"

	"github.com/isobit/ndog/internal"
	"github.com/isobit/ndog/internal/log"
)

type listenOptions struct {
	StatusCode    int
	Headers       map[string]string
	ServeFile     string
	MsgpackToJSON bool
}

var listenOptionHelp = ndog.OptionsHelp{}.
	Add("header.<NAME>", "<VALUE>", "extra response headers to send").
	Add("msgpack_to_json", "", "attempt to convert msgpack-encoded requests to JSON").
	Add("serve_file", "<PATH>", "use Go's ServeFile to serve files relative to this directory").
	Add("status_code", "<CODE>", "status code to send in response (default: 200)")

func extractListenOptions(opts ndog.Options) (listenOptions, error) {
	o := listenOptions{
		StatusCode: 200,
		Headers:    map[string]string{},
	}
	if _, ok := opts.Pop("msgpack_to_json"); ok {
		o.MsgpackToJSON = true
	}

	if val, ok := opts.Pop("status_code"); ok {
		if _, err := fmt.Sscanf(val, "%d", &val); err != nil {
			return o, fmt.Errorf("error parsing status_code option: %w", err)
		}
	}

	if serveFilePath, ok := opts.Pop("serve_file"); ok {
		serveFileAbsPath, err := filepath.Abs(serveFilePath)
		if err != nil {
			return o, fmt.Errorf("error parsing serve_file option: %w", err)
		}
		o.ServeFile = serveFileAbsPath
	}

	if proxyPass, ok := opts.Pop("proxy_pass"); ok {
		o.ProxyPass = proxyPass
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

func Listen(cfg ndog.ListenConfig) error {
	opts, err := extractListenOptions(cfg.Options)
	if err != nil {
		return err
	}
	if opts.ServeFile != "" {
		log.Logf(1, "http: will serve file(s) from %s", opts.ServeFile)
	}

	errLogReader, errLogWriter := io.Pipe()
	defer errLogWriter.Close()
	go func() {
		s := bufio.NewScanner(errLogReader)
		for s.Scan() {
			log.Logf(-1, s.Text())
		}
	}()

	s := &http.Server{
		ErrorLog: stdlog.New(errLogWriter, "", 0),
		Addr:     cfg.URL.Host,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Logf(0, "request: %s: %s %s", r.RemoteAddr, r.Method, r.URL)
			if r.Host != cfg.URL.Host {
				log.Logf(1, "request header: Host: %s", r.Host)
			}
			for key, values := range r.Header {
				log.Logf(1, "request header: %s: %s", key, strings.Join(values, ", "))
			}
			if opts.ServeFile != "" {
				http.ServeFile(w, r, filepath.Join(opts.ServeFile, r.URL.Path))
				return
			}

			stream := cfg.StreamManager.NewStream(fmt.Sprintf("%s|%s %s", r.RemoteAddr, r.Method, r.URL))
			defer stream.Close()

			// Receive request.
			contentType := r.Header.Get("Content-Type")
			if opts.MsgpackToJSON && (contentType == "application/msgpack" || contentType == "application/x-msgpack") {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					log.Logf(-1, "error reading request body: %s", err)
					return
				}
				if _, err := msgp.UnmarshalAsJSON(stream.Writer, body); err != nil {
					log.Logf(-1, "error unmarshaling request body msgpack as JSON: %s", err)
					return
				}
				stream.Writer.Write([]byte{'\n'})
			} else {
				if _, err := io.Copy(stream.Writer, r.Body); err != nil {
					log.Logf(-1, "error reading request body: %s", err)
					return
				}
				stream.Writer.Write([]byte{'\n'})
			}
			stream.Writer.Close()

			// Send response.
			for key, val := range opts.Headers {
				w.Header().Add(key, val)
			}
			log.Logf(10, "writing status code %d", opts.StatusCode)
			w.WriteHeader(opts.StatusCode)
			if _, err := io.Copy(w, stream.Reader); err != nil {
				log.Logf(-1, "error writing response body: %s", err)
				return
			}
			log.Logf(10, "handler closed")
		}),
	}
	if cfg.URL.Scheme == "https" {
		tlsConfig, err := cfg.TLS.Config(true, []string{cfg.URL.Hostname()})
		if err != nil {
			return err
		}
		s.TLSConfig = tlsConfig
		log.Logf(0, "listening: %s", s.Addr)
		return s.ListenAndServeTLS("", "")
	}
	log.Logf(0, "listening: %s", s.Addr)
	return s.ListenAndServe()
}
