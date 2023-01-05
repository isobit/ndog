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
	Log      bool

	ListenURL  *url.URL `cli:"name=listen,short=l,placeholder=URL"`
	ConnectURL *url.URL `cli:"name=connect,short=c,placeholder=URL"`

	Exec string `cli:"short=x,help=execute a command to handle streams"`
	Tee  bool   `cli:"short=t,help=also write command input to stdout"`

	FixedInput *string `cli:"short=F"`

	Options []string `cli:"short=o,name=option,append,placeholder=KEY=VAL,nodefault,help=scheme options; may be passed multiple times"`

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

	// Parse options.
	opts := map[string]string{}
	for _, s := range cmd.Options {
		key, value, _ := strings.Cut(s, "=")
		opts[key] = value
	}

	// if cmd.ListenURL != nil && cmd.ConnectURL != nil {
	// 	return cli.UsageErrorf("proxy (passing --listen and --connect) is not supported yet")
	// }

	var listenScheme *ndog.Scheme
	if cmd.ListenURL != nil {
		scheme, ok := schemes.Registry[cmd.ListenURL.Scheme]
		if !ok || scheme.Listen == nil {
			return fmt.Errorf("unknown listen scheme: %s", cmd.ListenURL.Scheme)
		}
		listenScheme = scheme
	}

	var connectScheme *ndog.Scheme
	if cmd.ConnectURL != nil {
		scheme, ok := schemes.Registry[cmd.ConnectURL.Scheme]
		if !ok || scheme.Connect == nil {
			return fmt.Errorf("unknown connect scheme: %s", cmd.ConnectURL.Scheme)
		}
		connectScheme = scheme
	}

	// var interactive bool
	var fixedInput []byte
	if cmd.FixedInput != nil {
		fixedInput = []byte(*cmd.FixedInput)
	}
	// else {
	// 	stdinStat, _ := os.Stdin.Stat()
	// 	interactive = stdinStat.Mode()&os.ModeCharDevice != 0
	// }

	var streamFactory ndog.StreamFactory
	switch {
	case listenScheme != nil && connectScheme != nil:
		streamFactory = ndog.ProxyStreamFactory{
			ConnectConfig: ndog.Config{
				Options: opts,
				URL:     cmd.ConnectURL,
			},
			Connect: connectScheme.Connect,
		}
	case cmd.Exec != "":
		args, err := shlex.Split(cmd.Exec)
		if err != nil {
			return cli.UsageErrorf("failed to split exec args: %s", err)
		}
		execStreamFactory := ndog.NewExecStreamFactory(args)
		if cmd.Tee {
			execStreamFactory.TeeWriter = os.Stdout
		}
		streamFactory = execStreamFactory
	// case interactive:
	// TODO
	default:
		streamFactory = ndog.NewStdIOStreamFactory(fixedInput)
	}
	if cmd.Log {
		streamFactory = ndog.NewLogStreamFactory(streamFactory)
	}

	switch {
	case listenScheme != nil:
		// return listenScheme.Listen(ndog.Config{
		// 	StreamFactory: streamFactory,
		// 	Options:       opts,
		// 	URL:           cmd.ListenURL,
		// })
		return listenScheme.Listen(ndog.ListenConfig{
			Config: ndog.Config{
				URL:     cmd.ListenURL,
				Options: opts,
			},
			StreamFactory: streamFactory,
		})
	case connectScheme != nil:
		// return connectScheme.Connect(ndog.Config{
		// 	StreamFactory: streamFactory,
		// 	Options:       opts,
		// 	URL:           cmd.ConnectURL,
		// })
		stream := streamFactory.NewStream("") // TODO name
		return connectScheme.Connect(ndog.ConnectConfig{
			Config: ndog.Config{
				URL:     cmd.ConnectURL,
				Options: opts,
			},
			Stream: stream,
		})
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
