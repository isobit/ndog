package main

import (
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/google/shlex"
	"github.com/isobit/cli"

	"github.com/isobit/ndog"
	"github.com/isobit/ndog/schemes"
)

func main() {
	if stderrStat, err := os.Stderr.Stat(); err == nil {
		if stderrStat.Mode()&os.ModeCharDevice != 0 {
			ndog.LogColor = true
		}
	}

	err := cli.New("ndog", &Ndog{}).
		Parse().
		Run()

	if err != nil && err != cli.ErrHelp {
		ndog.Logf(-1, "error: %s", err)
	}
}

type Ndog struct {
	Verbose  bool `cli:"short=v"`
	Debug    bool
	Quiet    bool `cli:"short=q"`
	LogLevel int  `cli:"hidden"`

	ListenURL  *url.URL `cli:"name=listen,short=l,placeholder=URL"`
	ConnectURL *url.URL `cli:"name=connect,short=c,placeholder=URL"`

	Exec        string   `cli:"short=x,help=execute a command to handle streams"`
	Options     []string `cli:"short=o,name=option,append,placeholder=KEY=VAL,nodefault,help=scheme options; may be passed multiple times"`
	Log         bool
	Interactive bool `cli:"short=i"`

	ListSchemes bool `cli:"help=list available schemes"`
}

func (cmd Ndog) Run() error {
	if cmd.ListSchemes {
		return listSchemes()
	}

	switch {
	case cmd.Verbose && cmd.Quiet:
		return cli.UsageErrorf("--verbose and --quiet are mutually exclusive")
	case cmd.LogLevel != 0:
		ndog.LogLevel = cmd.LogLevel
	case cmd.Quiet:
		ndog.LogLevel = -10
	case cmd.Verbose:
		ndog.LogLevel = 1
	case cmd.Debug:
		ndog.LogLevel = 10
	}

	var streamFactory ndog.StreamFactory
	if cmd.Exec != "" {
		args, err := shlex.Split(cmd.Exec)
		if err != nil {
			return cli.UsageErrorf("failed to split exec args: %s", err)
		}
		streamFactory = ndog.NewExecStreamFactory(args[0], args[1:]...)
	}
	if cmd.Interactive {
		// ndog.LogLevel = -10
		tui := ndog.NewTUI(streamFactory)
		streamFactory = tui
		originalLogf := ndog.Logf
		ndog.Logf = tui.Logf
		go func() {
			err := tui.Run()
			ndog.Logf = originalLogf
			if err != nil {
				ndog.Logf(-1, "error: %s", err)
				os.Exit(1)
			}
			os.Exit(0)
		}()
	}
	if streamFactory == nil {
		streamFactory = ndog.NewStdIOStreamFactory()
	}
	if cmd.Log || cmd.Interactive {
		streamFactory = ndog.NewLogStreamFactory(streamFactory)
	}

	// Parse options.
	opts := map[string]string{}
	for _, s := range cmd.Options {
		key, value, _ := strings.Cut(s, "=")
		opts[key] = value
	}

	cfg := ndog.Config{
		StreamFactory: streamFactory,
		Options:       opts,
	}

	switch {
	case cmd.ListenURL != nil && cmd.ConnectURL != nil:
		return cli.UsageErrorf("proxy (passing --listen and --connect) is not supported yet")
	case cmd.ListenURL != nil:
		cfg.URL = cmd.ListenURL
		if scheme, ok := schemes.Registry[cfg.URL.Scheme]; ok && scheme.Listen != nil {
			return scheme.Listen(cfg)
		} else {
			return fmt.Errorf("unknown listen scheme: %s", cfg.URL.Scheme)
		}

	case cmd.ConnectURL != nil:
		cfg.URL = cmd.ConnectURL
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
