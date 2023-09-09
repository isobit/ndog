package ndog

import (
	"fmt"
	"net/url"
	"strings"
)

type Scheme struct {
	Names []string

	Listen  func(ListenConfig) error
	Connect func(ConnectConfig) error

	Description       string
	ListenOptionHelp  []OptionHelp
	ConnectOptionHelp []OptionHelp
}

type OptionHelp struct {
	Name        string
	Value       string
	Description string
}

type Config struct {
	URL     *url.URL
	Options Options
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
