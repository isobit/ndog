package ndog

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/isobit/ndog/internal/netutil"
	ndog_tls "github.com/isobit/ndog/internal/tls"
)

type Scheme struct {
	Names       []string
	HiddenNames []string

	Listen  func(ListenConfig) error
	Connect func(ConnectConfig) error

	Description       string
	ListenOptionHelp  OptionsHelp
	ConnectOptionHelp OptionsHelp
}

type Config struct {
	URL     *url.URL
	Options Options
	TLS     ndog_tls.Config
	Net     netutil.Config
}

type ListenConfig struct {
	Config
	StreamManager StreamManager
}

type ConnectConfig struct {
	Config
	Stream Stream
}

type Options map[string]string

func (opts Options) Pop(key string) (string, bool) {
	v, ok := opts[key]
	if ok {
		delete(opts, key)
	}
	return v, ok
}

func (opts Options) Done() error {
	if len(opts) == 0 {
		return nil
	}
	keys := []string{}
	for k := range opts {
		keys = append(keys, k)
	}
	return fmt.Errorf("unknown options: %s", strings.Join(keys, ", "))
}

type OptionsHelp []OptionHelp

func (oh OptionsHelp) Add(name string, value string, description string) OptionsHelp {
	oh = append(oh, OptionHelp{
		Name:        name,
		Value:       value,
		Description: description,
	})
	return oh
}

type OptionHelp struct {
	Name        string
	Value       string
	Description string
}

func SplitURLSubscheme(url *url.URL) (*url.URL, string) {
	if url == nil {
		return nil, ""
	}
	urlCopy := *url
	scheme, subscheme, _ := strings.Cut(url.Scheme, "+")
	urlCopy.Scheme = scheme
	return &urlCopy, subscheme
}
