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

var Scheme = &ndog.Scheme{
	Names:   []string{"http"},
	Connect: Connect,
	Listen:  Listen,
}

func Listen(cfg ndog.Config) error {
	writeRequestLine := false
	if _, ok := cfg.Options["request_line"]; ok {
		writeRequestLine = true
	}
	msgpackToJSON := false
	if _, ok := cfg.Options["msgpack_to_json"]; ok {
		msgpackToJSON = true
	}

	headerKeyPrefix := "header."
	headers := map[string]string{}
	for key, val := range cfg.Options {
		if !strings.HasPrefix(key, headerKeyPrefix) {
			continue
		}
		headerKey := strings.TrimPrefix(key, headerKeyPrefix)
		headers[headerKey] = val
	}

	var statusCode int = 200
	if statusCodeOption, ok := cfg.Options["status_code"]; ok {
		if _, err := fmt.Sscanf(statusCodeOption, "%d", &statusCode); err != nil {
			return fmt.Errorf("error parsing status_code option: %w", err)
		}
	}

	var serveFile string
	if serveFilePath, ok := cfg.Options["serve_file"]; ok {
		serveFileAbsPath, err := filepath.Abs(serveFilePath)
		if err != nil {
			return fmt.Errorf("error parsing serve_file option: %w", err)
		}
		serveFile = serveFileAbsPath
		ndog.Logf(1, "http: will serve file(s) from %s", serveFile)
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

			if writeRequestLine {
				fmt.Fprintf(stream, "%s %s %s\n", r.Method, r.URL, r.Proto)
			}

			// Receive request.
			contentType := r.Header.Get("Content-Type")
			if msgpackToJSON && (contentType == "application/msgpack" || contentType == "application/x-msgpack") {
				body, err := ioutil.ReadAll(r.Body)
				if err != nil {
					ndog.Logf(-1, "error reading request body: %s", err)
					return
				}
				if _, err := msgp.UnmarshalAsJSON(stream, body); err != nil {
					ndog.Logf(-1, "error unmarshaling request body msgpack as JSON: %s", err)
					return
				}
			} else {
				io.Copy(stream, r.Body)
			}
			stream.CloseWriter()

			// Send response.
			if serveFile != "" {
				http.ServeFile(w, r, filepath.Join(serveFile, r.URL.Path))
			} else {
				for key, val := range headers {
					w.Header().Add(key, val)
				}
				w.WriteHeader(statusCode)
				io.Copy(w, stream)
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
	io.Copy(stream, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("got error response: %s", resp.Status)
	}
	return nil
}
