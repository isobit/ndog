package main

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

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
	Verbose     bool     `cli:"short=v"`
	ListenURL   *url.URL `cli:"name=listen,short=l,placeholder=URL"`
	ConnectURL  *url.URL `cli:"name=connect,short=c,placeholder=URL"`
	ListSchemes bool
}

func (cmd Ndog) Run() error {
	switch {
	case cmd.ListSchemes:
		return listSchemes()
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

func listSchemes() error {
	list := make([]string, len(schemes.Registry))
	i := 0
	for name := range schemes.Registry {
		list[i] = name
		i++
	}
	sort.Strings(list)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	for _, name := range list {
		scheme := schemes.Registry[name]
		supports := []string{}
		if scheme.Listen != nil {
			supports = append(supports, "listen")
		}
		if scheme.Connect != nil {
			supports = append(supports, "connect")
		}

		fmt.Fprintf(w, name)
		if len(supports) > 0 {
			fmt.Fprintf(w, "\t (%s)", strings.Join(supports, ", "))
		}
		fmt.Fprintln(w)
	}
	return w.Flush()
}
