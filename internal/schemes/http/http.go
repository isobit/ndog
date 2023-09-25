package http

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/tinylib/msgp/msgp"

	"github.com/isobit/ndog/internal"
)

var HTTPScheme = &ndog.Scheme{
	Names:   []string{"http"},
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

var HTTPSScheme = &ndog.Scheme{
	Names:   []string{"https"},
	Connect: Connect,

	Description: `
Connect sends input as an HTTPS request body to the specified URL.

Examples:
	GET request: ndog -c 'https://example.net/' -d ''
	`,
	ConnectOptionHelp: connectOptionHelp,
}

type listenOptions struct {
	StatusCode       int
	Headers          map[string]string
	ServeFile        string
	WriteRequestLine bool
	MsgpackToJSON    bool
	JSON             bool
}

var listenOptionHelp = ndog.OptionsHelp{}.
	Add("header.<NAME>", "<VALUE>", "extra response headers to send").
	Add("json", "", "use JSON representation for requests and responses").
	Add("msgpack_to_json", "", "attempt to convert msgpack-encoded requests to JSON").
	Add("serve_file", "<PATH>", "use Go's ServeFile to serve files relative to this directory").
	Add("status_code", "<CODE>", "status code to send in response (default: 200)")
	// {"request_line", "", ""}, // kinda weird, maybe should just drop this

func extractListenOptions(opts ndog.Options) (listenOptions, error) {
	o := listenOptions{
		StatusCode: 200,
		Headers:    map[string]string{},
	}
	if _, ok := opts.Pop("request_line"); ok {
		o.WriteRequestLine = true
	}
	if _, ok := opts.Pop("msgpack_to_json"); ok {
		o.MsgpackToJSON = true
	}
	if _, ok := opts.Pop("json"); ok {
		o.JSON = true
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

			if opts.JSON {
				jsonHandler(stream, w, r)
				return
			}

			if opts.WriteRequestLine {
				fmt.Fprintf(stream.Writer, "%s %s %s\n", r.Method, r.URL, r.Proto)
			}

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
				io.WriteString(stream.Writer, "\n")
			} else {
				if _, err := io.Copy(stream.Writer, r.Body); err != nil {
					ndog.Logf(-1, "error reading request body: %s", err)
					return
				}
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
	return s.ListenAndServe()
}

type connectOptions struct {
	Method  string
	Headers map[string]string
	JSON    bool
}

var connectOptionHelp = ndog.OptionsHelp{}.
	Add("header.<NAME>", "<VALUE>", "extra request headers to send").
	Add("json", "", "use JSON representation for requests and responses").
	Add("method", "<METHOD>", "HTTP method to use (default: GET)")

func extractConnectOptions(opts ndog.Options) (connectOptions, error) {
	o := connectOptions{
		Method:  "GET",
		Headers: map[string]string{},
	}
	if _, ok := opts.Pop("json"); ok {
		o.JSON = true
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

	return o, opts.Done()
}

func Connect(cfg ndog.ConnectConfig) error {
	opts, err := extractConnectOptions(cfg.Options)
	if err != nil {
		return err
	}
	if opts.JSON {
		return ConnectJSON(cfg)
	}

	// net/http hangs when io.Pipe is used as request body so collect it all
	// into a simple buffer reader.
	// https://github.com/golang/go/issues/29246
	body, err := io.ReadAll(cfg.Stream.Reader)
	if err != nil {
		return err
	}

	// Convert to HTTP request
	httpReq, err := http.NewRequest(opts.Method, cfg.URL.String(), bytes.NewReader(body))
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

	// Do request
	ndog.Logf(0, "request: %s %s", opts.Method, cfg.URL.RequestURI())
	for key, values := range httpReq.Header {
		ndog.Logf(1, "request header: %s: %s", key, strings.Join(values, ", "))
	}
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return err
	}

	ndog.Logf(0, "response: %s", resp.Status)
	for key, values := range resp.Header {
		ndog.Logf(1, "response header: %s: %s", key, strings.Join(values, ", "))
	}
	if _, err := io.Copy(cfg.Stream.Writer, resp.Body); err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf(resp.Status)
	}
	return nil
}

func ConnectJSON(cfg ndog.ConnectConfig) error {
	ndog.Logf(0, "request: GET %s", cfg.URL.RequestURI())

	stream := cfg.Stream

	// Read and decode request from stream
	req, err := ndog.ReadJSON[requestData](stream)
	if err != nil {
		return err
	}

	// Convert to HTTP request
	method := req.Method
	if method == "" {
		method = "GET"
	}
	url := req.URL
	if url == "" {
		url = cfg.URL.String()
	}
	httpReq, err := http.NewRequest(method, url, bytes.NewBufferString(req.Body))
	if err != nil {
		return err
	}
	if req.Proto != "" {
		httpReq.Proto = req.Proto
	}
	for key, vals := range req.Headers {
		for _, val := range vals {
			httpReq.Header.Add(key, val)
		}
	}

	// Do request
	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return err
	}

	ndog.Logf(0, "response: %s", httpResp.Status)

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return err
	}
	httpResp.Body.Close()

	resp := responseData{
		StatusCode: httpResp.StatusCode,
		Body:       string(body),
		Headers:    map[string][]string{},
	}
	for key, values := range httpResp.Header {
		resp.Headers[key] = values
	}
	if err := ndog.WriteJSON(stream, resp); err != nil {
		return err
	}

	return nil
}

type requestData struct {
	Proto   string
	Method  string
	URL     string
	Headers map[string][]string
	Body    string
}

type responseData struct {
	StatusCode int
	Headers    map[string][]string
	Body       string
}

func jsonHandler(stream ndog.Stream, w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		ndog.Logf(-1, "error reading request body: %s", err)
		w.WriteHeader(500)
		return
	}

	if err := ndog.WriteJSON(stream, requestData{
		Proto:   r.Proto,
		Method:  r.Method,
		URL:     r.URL.String(),
		Headers: r.Header,
		Body:    string(body),
	}); err != nil {
		ndog.Logf(-1, "error writing request JSON: %s", err)
		w.WriteHeader(500)
		return
	}

	resp, err := ndog.ReadJSON[responseData](stream)
	if err != nil {
		ndog.Logf(-1, "error reading response JSON: %s", err)
		w.WriteHeader(500)
		return
	}

	header := w.Header()
	for key, vals := range resp.Headers {
		for _, val := range vals {
			header.Add(key, val)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.WriteString(w, resp.Body)
	ndog.Logf(10, "handler closed")
}