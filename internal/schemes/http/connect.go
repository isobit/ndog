package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/isobit/ndog/internal"
	"github.com/isobit/ndog/internal/log"
)

type connectOptions struct {
	Method          string
	Headers         map[string]string
	FollowRedirects bool
	GraphQL         bool
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

	switch subscheme {
	case "graphql":
		o.GraphQL = true
		o.Method = "POST"
		o.Headers["Content-Type"] = "application/json"
	default:
		if m := strings.ToUpper(subscheme); slices.Contains(methods, m) {
			o.Method = m
		}
	}

	if val, ok := opts.Pop("method"); ok {
		o.Method = strings.ToUpper(val)
	}

	if _, ok := opts.Pop("follow_redirects"); ok {
		o.FollowRedirects = true
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
			log.Logf(2, "setting host: %s", val)
			httpReq.Host = val
		}
		httpReq.Header.Add(key, val)
	}

	tlsConfig, err := cfg.TLS.Config(false, nil)
	if err != nil {
		return err
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	client := &http.Client{
		Transport: transport,
	}
	if !opts.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	// Do request
	log.Logf(0, "request: %s %s", opts.Method, reqUrl.RequestURI())
	for key, values := range httpReq.Header {
		log.Logf(1, "request header: %s: %s", key, strings.Join(values, ", "))
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}

	log.Logf(0, "response: %s", resp.Status)
	for key, values := range resp.Header {
		log.Logf(1, "response header: %s: %s", key, strings.Join(values, ", "))
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
