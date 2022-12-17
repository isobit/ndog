package ndog

import (
	"fmt"
	"net/url"
	"strings"
)

type Config struct {
	StreamFactory
	Options map[string]string
	URL     *url.URL
}

func (cfg *Config) PopOption(key string) (string, bool) {
	v, ok := cfg.Options[key]
	if ok {
		delete(cfg.Options, key)
	}
	return v, ok
}

func (cfg *Config) CheckRemainingOptions() error {
	if len(cfg.Options) == 0 {
		return nil
	}
	keys := []string{}
	for k := range cfg.Options {
		keys = append(keys, k)
	}
	return fmt.Errorf("unknown options: %s", strings.Join(keys, ", "))
}

type Scheme struct {
	Names   []string
	Listen  func(Config) error
	Connect func(Config) error
}
