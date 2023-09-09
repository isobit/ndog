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
	ListenURL  *url.URL `cli:"name=listen,short=l,placeholder=URL"`
	ConnectURL *url.URL `cli:"name=connect,short=c,placeholder=URL"`

	Options []string `cli:"short=o,name=option,append,placeholder=KEY=VAL,nodefault,help=scheme options; may be passed multiple times"`

	Data *string `cli:"short=d,help=use specified data instead of reading from STDIN"`
	Exec string  `cli:"short=x,help=execute a command to handle streams"`
	Tee  bool    `cli:"short=t,help=also write command input to stdout"`

	ListSchemes bool   `cli:"short=L,help=list available schemes"`
	SchemeHelp  string `cli:"short=H,help=show help for scheme"`

	Verbose  bool `cli:"short=v,help=more verbose logging"`
	Quiet    bool `cli:"short=q,help=disable all logging"`
	Debug    bool `cli:"help=maximum logging"`
	LogLevel int  `cli:"hidden"`
	LogIO    bool `cli:"help=log all I/O"`
}

func (cmd Ndog) Run() error {
	if cmd.ListSchemes {
		return listSchemes()
	}
	if cmd.SchemeHelp != "" {
		return schemeHelp(cmd.SchemeHelp)
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

	var listenScheme *ndog.Scheme
	if cmd.ListenURL != nil {
		scheme := schemes.Lookup(cmd.ListenURL.Scheme)
		if scheme == nil || scheme.Listen == nil {
			return fmt.Errorf("unknown listen scheme: %s", cmd.ListenURL.Scheme)
		}
		listenScheme = scheme
	}

	var connectScheme *ndog.Scheme
	if cmd.ConnectURL != nil {
		scheme := schemes.Lookup(cmd.ConnectURL.Scheme)
		if scheme == nil || scheme.Connect == nil {
			return fmt.Errorf("unknown connect scheme: %s", cmd.ConnectURL.Scheme)
		}
		connectScheme = scheme
	}

	// var interactive bool
	var fixedData []byte
	if cmd.Data != nil {
		fixedData = []byte(*cmd.Data)
	}
	// else {
	// 	stdinStat, _ := os.Stdin.Stat()
	// 	interactive = stdinStat.Mode()&os.ModeCharDevice != 0
	// }

	var streamManager ndog.StreamManager
	switch {
	case listenScheme != nil && connectScheme != nil:
		streamManager = ndog.ProxyStreamManager{
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
		execStreamManager := ndog.NewExecStreamManager(args)
		if cmd.Tee {
			execStreamManager.TeeWriter = os.Stdout
		}
		streamManager = execStreamManager
	// case interactive:
	// TODO
	default:
		streamManager = ndog.NewStdIOStreamManager(fixedData)
	}
	if cmd.LogIO {
		streamManager = ndog.NewLogStreamManager(streamManager)
	}

	switch {
	case listenScheme != nil:
		return listenScheme.Listen(ndog.ListenConfig{
			Config: ndog.Config{
				URL:     cmd.ListenURL,
				Options: opts,
			},
			StreamManager: streamManager,
		})
	case connectScheme != nil:
		stream := streamManager.NewStream(cmd.ConnectURL.String())
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

	w := tabwriter.NewWriter(os.Stderr, 0, 0, 1, ' ', 0)
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

func schemeHelp(name string) error {
	scheme, ok := schemes.Registry[name]
	if !ok {
		return fmt.Errorf("unknown scheme: %s; use --list-schemes to show valid schemes", name)
	}

	w := tabwriter.NewWriter(os.Stderr, 0, 0, 1, ' ', 0)

	fmt.Fprintf(w, "SCHEME: %s\n", strings.Join(scheme.Names, " "))
	fmt.Fprintln(w)

	if scheme.Description != "" {
		fmt.Fprintf(w, "DESCRIPTION:\n")
		fmt.Fprintf(w, "    %s\n", strings.ReplaceAll(strings.TrimSpace(scheme.Description), "\n", "\n    "))
		fmt.Fprintln(w)
	}

	if scheme.ListenOptionHelp != nil {
		fmt.Fprintf(w, "LISTEN OPTIONS:\n")
		for _, h := range scheme.ListenOptionHelp {
			fmt.Fprintf(w, "    %s\t%s\t%s\t\n", h.Name, h.Value, h.Description)
		}
		fmt.Fprintln(w)
	}

	if scheme.ConnectOptionHelp != nil {
		fmt.Fprintf(w, "CONNECT OPTIONS:\n")
		for _, h := range scheme.ConnectOptionHelp {
			fmt.Fprintf(w, "    %s\t%s\t%s\t\n", h.Name, h.Value, h.Description)
		}
		fmt.Fprintln(w)
	}

	return w.Flush()
}
