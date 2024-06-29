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
	"github.com/isobit/ndog/internal/util"
	"github.com/isobit/ndog/internal/version"
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
		Method: "GET",
		Headers: map[string]string{
			"User-Agent": fmt.Sprintf("ndog/%s", version.Version),
		},
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

	var body io.Reader
	if opts.GraphQL {
		queryData, err := io.ReadAll(cfg.Stream.Reader)
		if err != nil {
			return err
		}
		bodyData, err := json.Marshal(GraphQLRequest{
			Query: string(queryData),
		})
		if err != nil {
			return err
		}
		body = bytes.NewReader(bodyData)
	} else if opts.Method != "GET" {
		// MDN's docs on GET explain of sending request bodies with GET, "while not
		// prohibited by the specification, the semantics are undefined. It is
		// better to just avoid sending payloads in GET requests". Since request
		// bodies really shouldn't be sent with GET anyway, we can avoid hanging
		// waiting for data on stdin (if the user doesn't close stdin or specify
		// static empty data as a CLI argument) by only reading the input stream on
		// non-GET requests.
		body = cfg.Stream.Reader
	}

	// Convert to HTTP request
	httpReq, err := http.NewRequest(opts.Method, reqUrl.String(), body)
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
	util.LogHeaders("request header: ", httpReq.Header)
	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}

	log.Logf(0, "response: %s", resp.Status)
	util.LogHeaders("response header: ", resp.Header)

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
