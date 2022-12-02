package main

import (
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/isobit/cli"

	"github.com/isobit/ndog"
	"github.com/isobit/ndog/schemes"
)

var in io.Reader = os.Stdin
var log io.Writer = os.Stderr
var out io.Writer = os.Stdout

var verbose bool = false
var logColor bool = false

func logf(format string, v ...interface{}) (int, error) {
	if logColor {
		format = "\u001b[30;1m" + format + "\u001b[0m"
	}
	if len(format) > 0 && format[len(format)-1] != '\n' {
		format = format + "\n"
	}
	return fmt.Fprintf(log, format, v...)
}

func main() {
	if stderrStat, err := os.Stderr.Stat(); err == nil {
		if stderrStat.Mode()&os.ModeCharDevice != 0 {
			logColor = true
		}
	}

	cli.New("ndog", &Ndog{}).
		Parse().
		RunFatal()
}

type Ndog struct {
	Verbose    bool     `cli:"short=v"`
	ListenURL  *url.URL `cli:"name=listen,short=l,placeholder=URL"`
	ConnectURL *url.URL `cli:"name=connect,short=c,placeholder=URL"`
}

func (cmd Ndog) Run() error {
	switch {
	case cmd.ListenURL != nil && cmd.ConnectURL != nil:
		return fmt.Errorf("proxy is not supported yet")

	case cmd.ListenURL != nil:
		cfg := ndog.Config{
			Verbose: cmd.Verbose,
			URL:     cmd.ListenURL,
			In:      in,
			Out:     out,
			Logf:    logf,
		}
		if scheme, ok := schemes.Registry[cfg.URL.Scheme]; ok && scheme.Listen != nil {
			return scheme.Listen(cfg)
		} else {
			return fmt.Errorf("unknown listen scheme: %s", cfg.URL.Scheme)
		}

	case cmd.ConnectURL != nil:
		cfg := ndog.Config{
			Verbose: cmd.Verbose,
			URL:     cmd.ConnectURL,
			In:      in,
			Out:     out,
			Logf:    logf,
		}
		if scheme, ok := schemes.Registry[cfg.URL.Scheme]; ok && scheme.Connect != nil {
			return scheme.Connect(cfg)
		} else {
			return fmt.Errorf("unknown connect scheme: %s", cfg.URL.Scheme)
		}
	default:
		return cli.UsageErrorf("at least one of --listen or --connect must be specified")
	}
}
