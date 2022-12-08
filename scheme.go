package ndog

import (
	"net/url"
)

type Config struct {
	StreamFactory
	Options map[string]string
	URL     *url.URL
}

type Scheme struct {
	Names   []string
	Listen  func(Config) error
	Connect func(Config) error
}
