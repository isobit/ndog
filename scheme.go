package ndog

import (
	"io"
	"net/url"
)

type Config struct {
	Verbose bool
	Options map[string]string
	URL     *url.URL

	In   io.Reader
	Out  io.Writer
	Logf func(format string, v ...interface{}) (int, error)
	// IO      IOHandler
}

// type IOHandler interface {

// }

type Scheme struct {
	Names   []string
	Listen  func(Config) error
	Connect func(Config) error
}
