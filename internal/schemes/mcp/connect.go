package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	ndog "github.com/isobit/ndog/internal"
	"github.com/isobit/ndog/internal/log"
	ndog_version "github.com/isobit/ndog/internal/version"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type connectOptions struct {
	JSON    bool
	Timeout time.Duration
	Params  map[string]any
	Headers map[string][]string
}

func extractConnectOptions(opts ndog.Options) (connectOptions, error) {
	o := connectOptions{
		Timeout: 30 * time.Second,
		Params:  map[string]any{},
		Headers: map[string][]string{},
	}

	if _, ok := opts.Pop("json"); ok {
		o.JSON = true
	}

	if val, ok := opts.Pop("timeout"); ok {
		d, err := time.ParseDuration(val)
		if err != nil {
			return o, fmt.Errorf("invalid timeout: %w", err)
		}
		o.Timeout = d
	}

	paramPrefix := "param."
	for key, vals := range opts {
		if !strings.HasPrefix(key, paramPrefix) {
			continue
		}
		paramKey := strings.TrimPrefix(key, paramPrefix)
		if len(vals) == 1 {
			o.Params[paramKey] = vals[0]
		} else {
			o.Params[paramKey] = vals
		}
		delete(opts, key)
	}

	headerPrefix := "header."
	for key, vals := range opts {
		if !strings.HasPrefix(key, headerPrefix) {
			continue
		}
		headerKey := strings.TrimPrefix(key, headerPrefix)
		o.Headers[headerKey] = append(o.Headers[headerKey], vals...)
		delete(opts, key)
	}

	return o, opts.Done()
}

func mcpURLToEndpoint(mcpURL *url.URL) string {
	reqURL, subscheme := ndog.SplitURLSubscheme(mcpURL)
	if subscheme == "" {
		subscheme = "http"
	}

	endpointURL := url.URL{
		Scheme:   subscheme,
		Host:     reqURL.Host,
		User:     reqURL.User,
		Path:     reqURL.Path,
		RawQuery: reqURL.RawQuery,
	}
	return endpointURL.String()
}

func Connect(cfg ndog.ConnectConfig) error {
	opts, err := extractConnectOptions(cfg.Options)
	if err != nil {
		return err
	}

	endpoint := mcpURLToEndpoint(cfg.URL)
	fragment := cfg.URL.Fragment

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	session, err := connect(ctx, endpoint, opts.Headers)
	if err != nil {
		return err
	}
	defer session.Close()

	op, target := parseOperation(fragment)

	switch op {
	case "tools/list":
		return toolsList(ctx, session, cfg.Stream, opts)
	case "tools/call":
		return toolsCall(ctx, session, cfg.Stream, opts, target)
	case "resources/list":
		return resourcesList(ctx, session, cfg.Stream, opts)
	case "resources/read":
		return resourcesRead(ctx, session, cfg.Stream, opts, target)
	case "prompts/list":
		return promptsList(ctx, session, cfg.Stream, opts)
	case "prompts/get":
		return promptsGet(ctx, session, cfg.Stream, opts, target)
	case "":
		_, err := fmt.Fprintln(cfg.Stream.Writer, session.InitializeResult().Instructions)
		return err
	default:
		return fmt.Errorf("unknown operation: %s", op)
	}
}

func parseOperation(fragment string) (op, target string) {
	if fragment == "" {
		return "", ""
	}

	// Check for known operation prefixes.
	prefixes := []string{
		"tools/list",
		"tools/call/",
		"resources/list",
		"resources/read/",
		"prompts/list",
		"prompts/get/",
	}
	for _, p := range prefixes {
		if fragment == strings.TrimSuffix(p, "/") {
			return fragment, ""
		}
		if after, ok := strings.CutPrefix(fragment, p); ok {
			return strings.TrimSuffix(p, "/"), after
		}
	}

	// Default: treat as tool call shorthand.
	return "tools/call", fragment
}

// headerTransport injects headers into every outgoing request and delegates
// to an underlying RoundTripper.
type headerTransport struct {
	base    http.RoundTripper
	headers map[string][]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for key, vals := range t.headers {
		for _, val := range vals {
			req.Header.Add(key, val)
		}
	}
	return t.base.RoundTrip(req)
}

// statusCapturingTransport wraps another RoundTripper to capture the HTTP
// status code from the first response, giving us structured error info
// even though the SDK only returns string-formatted errors.
type statusCapturingTransport struct {
	base   http.RoundTripper
	once   sync.Once
	status int
}

func (t *statusCapturingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err == nil {
		t.once.Do(func() {
			t.status = resp.StatusCode
		})
	}
	return resp, err
}

func newHTTPClient(headers map[string][]string) (*http.Client, *statusCapturingTransport) {
	var base http.RoundTripper = http.DefaultTransport
	if len(headers) > 0 {
		base = &headerTransport{base: base, headers: headers}
	}
	sc := &statusCapturingTransport{base: base}
	return &http.Client{Transport: sc}, sc
}

func connect(ctx context.Context, endpoint string, headers map[string][]string) (*mcp.ClientSession, error) {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "ndog",
		Version: ndog_version.Version,
	}, nil)

	// Try streamable HTTP first.
	log.Logf(1, "connecting (streamable): %s", endpoint)
	httpClient, statusTransport := newHTTPClient(headers)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   endpoint,
		HTTPClient: httpClient,
	}, nil)
	if err == nil {
		log.Logf(0, "connected: %s", endpoint)
		return session, nil
	}
	streamableErr := err
	log.Logf(1, "streamable transport failed: %s", err)

	// If we got a definitive HTTP error (auth, server error), don't bother
	// falling back to SSE — the server understood our request and rejected it.
	if s := statusTransport.status; s == http.StatusUnauthorized ||
		s == http.StatusForbidden ||
		s == http.StatusInternalServerError ||
		s == http.StatusServiceUnavailable {
		return nil, fmt.Errorf("connecting to %s: HTTP %d: %w", endpoint, s, streamableErr)
	}

	// Fall back to legacy SSE.
	log.Logf(1, "connecting (sse): %s", endpoint)
	sseClient, _ := newHTTPClient(headers)
	session, err = client.Connect(ctx, &mcp.SSEClientTransport{
		Endpoint:   endpoint,
		HTTPClient: sseClient,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: streamable: %v; sse: %w", endpoint, streamableErr, err)
	}
	log.Logf(0, "connected (sse): %s", endpoint)
	return session, nil
}

func toolsList(ctx context.Context, session *mcp.ClientSession, stream ndog.Stream, opts connectOptions) error {
	var allTools []*mcp.Tool
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			return err
		}
		allTools = append(allTools, tool)
	}

	if opts.JSON {
		return writeJSON(stream.Writer, allTools)
	}

	for _, tool := range allTools {
		name := tool.Name
		if tool.Description != "" {
			fmt.Fprintf(stream.Writer, "%s - %s\n", name, tool.Description)
		} else {
			fmt.Fprintln(stream.Writer, name)
		}
	}
	return nil
}

func toolsCall(ctx context.Context, session *mcp.ClientSession, stream ndog.Stream, opts connectOptions, toolName string) error {
	if toolName == "" {
		return fmt.Errorf("tool name required")
	}

	args, err := buildArgs(stream.Reader, opts.Params)
	if err != nil {
		return err
	}

	log.Logf(0, "calling tool: %s", toolName)
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		return err
	}

	if opts.JSON {
		return writeJSON(stream.Writer, result)
	}

	for _, content := range result.Content {
		switch c := content.(type) {
		case *mcp.TextContent:
			fmt.Fprint(stream.Writer, c.Text)
		default:
			data, err := json.Marshal(content)
			if err != nil {
				return err
			}
			stream.Writer.Write(data)
		}
	}

	if result.IsError {
		return fmt.Errorf("tool returned error")
	}
	return nil
}

func resourcesList(ctx context.Context, session *mcp.ClientSession, stream ndog.Stream, opts connectOptions) error {
	var allResources []*mcp.Resource
	for resource, err := range session.Resources(ctx, nil) {
		if err != nil {
			return err
		}
		allResources = append(allResources, resource)
	}

	if opts.JSON {
		return writeJSON(stream.Writer, allResources)
	}

	for _, r := range allResources {
		if r.Description != "" {
			fmt.Fprintf(stream.Writer, "%s - %s\n", r.URI, r.Description)
		} else {
			fmt.Fprintln(stream.Writer, r.URI)
		}
	}
	return nil
}

func resourcesRead(ctx context.Context, session *mcp.ClientSession, stream ndog.Stream, opts connectOptions, uri string) error {
	if uri == "" {
		return fmt.Errorf("resource URI required")
	}

	log.Logf(0, "reading resource: %s", uri)
	result, err := session.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: uri,
	})
	if err != nil {
		return err
	}

	if opts.JSON {
		return writeJSON(stream.Writer, result)
	}

	for _, content := range result.Contents {
		if content.Text != "" {
			fmt.Fprint(stream.Writer, content.Text)
		} else if content.Blob != nil {
			stream.Writer.Write(content.Blob)
		}
	}
	return nil
}

func promptsList(ctx context.Context, session *mcp.ClientSession, stream ndog.Stream, opts connectOptions) error {
	var allPrompts []*mcp.Prompt
	for prompt, err := range session.Prompts(ctx, nil) {
		if err != nil {
			return err
		}
		allPrompts = append(allPrompts, prompt)
	}

	if opts.JSON {
		return writeJSON(stream.Writer, allPrompts)
	}

	for _, p := range allPrompts {
		if p.Description != "" {
			fmt.Fprintf(stream.Writer, "%s - %s\n", p.Name, p.Description)
		} else {
			fmt.Fprintln(stream.Writer, p.Name)
		}
	}
	return nil
}

func promptsGet(ctx context.Context, session *mcp.ClientSession, stream ndog.Stream, opts connectOptions, name string) error {
	if name == "" {
		return fmt.Errorf("prompt name required")
	}

	log.Logf(0, "getting prompt: %s", name)
	result, err := session.GetPrompt(ctx, &mcp.GetPromptParams{
		Name: name,
	})
	if err != nil {
		return err
	}

	if opts.JSON {
		return writeJSON(stream.Writer, result)
	}

	for _, msg := range result.Messages {
		switch c := msg.Content.(type) {
		case *mcp.TextContent:
			fmt.Fprint(stream.Writer, c.Text)
		default:
			data, err := json.Marshal(msg.Content)
			if err != nil {
				return err
			}
			stream.Writer.Write(data)
		}
	}
	return nil
}

func buildArgs(reader io.ReadCloser, params map[string]any) (map[string]any, error) {
	args := map[string]any{}

	// Read stdin JSON if available.
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &args); err != nil {
			return nil, fmt.Errorf("invalid JSON input: %w", err)
		}
	}

	// Overlay params (higher priority).
	maps.Copy(args, params)

	if len(args) == 0 {
		return nil, nil
	}
	return args, nil
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
