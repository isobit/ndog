package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"os"
	"crypto/x509"
	"crypto/tls"

	"github.com/tinylib/msgp/msgp"

	"github.com/isobit/ndog/internal"
)

var HTTPScheme = &ndog.Scheme{
	Names:   []string{"http", "https"},
	Connect: Connect,
	Listen:  Listen,

	Description: `
Connect sends input as an HTTP request body to the specified URL.

Listen sends input as an HTTP request body to the specified URL.

Examples:
	GET request: ndog -c 'http://example.net/' -d ''
	Echo server: ndog -l 'http://localhost:8080' -x 'cat'
	File server: ndog -l 'http://localhost:8080' -o 'serve_file=.'
	`,
	ConnectOptionHelp: connectOptionHelp,
	ListenOptionHelp:  listenOptionHelp,
}

var HTTPGraphQLScheme = &ndog.Scheme{
	Names:   []string{"http+graphql", "https+graphql"},
	Connect: Connect,

	Description: `
Connect sends input as an GraphQL query to the specified URL over an HTTP POST request.

Examples:
	./ndog -c https+graphql://countries.trevorblades.com/graphql -d 'query { countries { name } }'
	`,
	ConnectOptionHelp: connectOptionHelpGraphql,
}

type listenOptions struct {
	StatusCode    int
	Headers       map[string]string
	ServeFile     string
	MsgpackToJSON bool

	TLSCert string
	TLSKey string
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

	headerKeyPrefix := "header."
	for key, val := range opts {
		if !strings.HasPrefix(key, headerKeyPrefix) {
			continue
		}
		headerKey := strings.TrimPrefix(key, headerKeyPrefix)
		o.Headers[headerKey] = val
		delete(opts, key)
	}

	if val, ok := opts.Pop("cert"); ok {
		o.TLSCert = val
	}
	if val, ok := opts.Pop("key"); ok {
		o.TLSKey = val
	}

	return o, opts.Done()
}

func Listen(cfg ndog.ListenConfig) error {
	opts, err := extractListenOptions(cfg.Options)
	if err != nil {
		return err
	}
	if opts.ServeFile != "" {
		ndog.Logf(1, "http: will serve file(s) from %s", opts.ServeFile)
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
			if opts.ServeFile != "" {
				http.ServeFile(w, r, filepath.Join(opts.ServeFile, r.URL.Path))
				return
			}

			stream := cfg.StreamManager.NewStream(fmt.Sprintf("%s|%s %s", r.RemoteAddr, r.Method, r.URL))
			defer stream.Close()

			// Receive request.
			contentType := r.Header.Get("Content-Type")
			if opts.MsgpackToJSON && (contentType == "application/msgpack" || contentType == "application/x-msgpack") {
				body, err := ioutil.ReadAll(r.Body)
				if err != nil {
					ndog.Logf(-1, "error reading request body: %s", err)
					return
				}
				if _, err := msgp.UnmarshalAsJSON(stream.Writer, body); err != nil {
					ndog.Logf(-1, "error unmarshaling request body msgpack as JSON: %s", err)
					return
				}
				stream.Writer.Write([]byte{'\n'})
			} else {
				if _, err := io.Copy(stream.Writer, r.Body); err != nil {
					ndog.Logf(-1, "error reading request body: %s", err)
					return
				}
				stream.Writer.Write([]byte{'\n'})
			}
			stream.Writer.Close()

			// Send response.
			for key, val := range opts.Headers {
				w.Header().Add(key, val)
			}
			ndog.Logf(10, "writing status code %d", opts.StatusCode)
			w.WriteHeader(opts.StatusCode)
			if _, err := io.Copy(w, stream.Reader); err != nil {
				ndog.Logf(-1, "error writing response body: %s", err)
				return
			}
			ndog.Logf(10, "handler closed")
		}),
	}
	ndog.Logf(0, "listening: %s", s.Addr)
	if cfg.URL.Scheme == "https" {
		return s.ListenAndServeTLS(opts.TLSCert, opts.TLSKey)
	}
	return s.ListenAndServe()
}

type connectOptions struct {
	Method  string
	Headers map[string]string
	CACert  string
	GraphQL bool
}

var connectOptionHelp = ndog.OptionsHelp{}.
	Add("header.<NAME>", "<VALUE>", "extra request headers to send").
	Add("method", "<METHOD>", "HTTP method to use (default: GET)")

var connectOptionHelpGraphql = ndog.OptionsHelp{}.
	Add("header.<NAME>", "<VALUE>", "extra request headers to send").
	Add("method", "<METHOD>", "HTTP method to use (default: POST)")

func extractConnectOptions(opts ndog.Options, subscheme string) (connectOptions, error) {
	o := connectOptions{
		Method:  "GET",
		Headers: map[string]string{},
	}

	if subscheme == "graphql" {
		o.GraphQL = true
		o.Method = "POST"
		o.Headers["Content-Type"] = "application/json"
	}

	if val, ok := opts.Pop("method"); ok {
		o.Method = strings.ToUpper(val)
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

	if val, ok := opts.Pop("cacert"); ok {
		o.CACert = val
	}

	return o, opts.Done()
}

func Connect(cfg ndog.ConnectConfig) error {
	reqUrl, subscheme := ndog.SplitURLSubscheme(cfg.URL)

	opts, err := extractConnectOptions(cfg.Options, subscheme)
	if err != nil {
		return err
	}

	// net/http hangs when io.Pipe is used as request body so collect it all
	// into a simple buffer reader.
	// https://github.com/golang/go/issues/29246
	body, err := io.ReadAll(cfg.Stream.Reader)
	if err != nil {
		return err
	}

	if opts.GraphQL {
		bodyJson, err := json.Marshal(GraphQLRequest{
			Query: string(body),
		})
		if err != nil {
			return err
		}
		body = bodyJson
	}

	// Convert to HTTP request
	httpReq, err := http.NewRequest(opts.Method, reqUrl.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	for key, val := range opts.Headers {
		if strings.EqualFold(key, "host") {
			ndog.Logf(2, "setting host: %s", val)
			httpReq.Host = val
		}
		httpReq.Header.Add(key, val)
	}

	client := &http.Client{}
	if opts.CACert != "" {
		cert, err := os.ReadFile(opts.CACert)
		if err != nil {
			return fmt.Errorf("error reading CA cert %s: %w", opts.CACert, err)
		}
		certPool := x509.NewCertPool()
		if ok := certPool.AppendCertsFromPEM(cert); !ok {
			return fmt.Errorf("unable to parse CA cert %s", opts.CACert)
		}
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certPool,
			},
		}
	}

	// Do request
	ndog.Logf(0, "request: %s %s", opts.Method, reqUrl.RequestURI())
	for key, values := range httpReq.Header {
		ndog.Logf(1, "request header: %s: %s", key, strings.Join(values, ", "))
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}

	ndog.Logf(0, "response: %s", resp.Status)
	for key, values := range resp.Header {
		ndog.Logf(1, "response header: %s: %s", key, strings.Join(values, ", "))
	}

	if opts.GraphQL {
		var bodyJson GraphQLResponse
		if err := json.NewDecoder(resp.Body).Decode(&bodyJson); err != nil {
			return fmt.Errorf("error decoding response: %w", err)
		}
		if bodyJson.Errors != nil && len(bodyJson.Errors) > 0 {
			errorJson, err := json.Marshal(bodyJson.Errors)
			if err != nil {
				return err
			}
			return fmt.Errorf("errors in GraphQL response: %s", string(errorJson))
		}
		if err := json.NewEncoder(cfg.Stream.Writer).Encode(bodyJson.Data); err != nil {
			return err
		}
	} else {
		if _, err := io.Copy(cfg.Stream.Writer, resp.Body); err != nil {
			return err
		}
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf(resp.Status)
	}
	return nil
}
