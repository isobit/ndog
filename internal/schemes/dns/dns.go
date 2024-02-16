package dns

import (
	"context"
	// "errors"
	"fmt"
	// "io"
	"net"
	"strings"

	"github.com/isobit/ndog/internal"
	// "github.com/isobit/ndog/internal/log"
)

var Scheme = &ndog.Scheme{
	Names:   []string{"dns"},
	Connect: Connect,
}

func Connect(cfg ndog.ConnectConfig) error {
	ctx := context.TODO()
	r := net.Resolver{}

	if cfg.URL.Host != "" {
		return fmt.Errorf("specifying a nameserver is not supported yet")
	}

	path := strings.SplitN(strings.TrimPrefix(cfg.URL.Path, "/"), "/", 2)

	host := path[0]

	var typ string
	if len(path) == 2 {
		typ = path[1]
	}

	switch typ {
	case "", "A", "AAAA":
		var network string
		switch typ {
		case "":
			network = "ip"
		case "A":
			network = "ip4"
		case "AAAA":
			network = "ip6"
		}
		recs, err := r.LookupIP(ctx, network, host)
		if err != nil {
			return err
		}
		for _, rec := range recs {
			fmt.Fprintln(cfg.Stream.Writer, rec.String())
		}
	case "CNAME":
		rec, err := r.LookupCNAME(ctx, host)
		if err != nil {
			return err
		}
		fmt.Fprintln(cfg.Stream.Writer, rec)
	case "TXT":
		recs, err := r.LookupTXT(ctx, host)
		if err != nil {
			return err
		}
		for _, rec := range recs {
			fmt.Fprintln(cfg.Stream.Writer, rec)
		}
	case "NS":
		recs, err := r.LookupNS(ctx, host)
		if err != nil {
			return err
		}
		for _, rec := range recs {
			fmt.Fprintln(cfg.Stream.Writer, rec.Host)
		}
	case "MX":
		recs, err := r.LookupMX(ctx, host)
		if err != nil {
			return err
		}
		for _, rec := range recs {
			fmt.Fprintf(cfg.Stream.Writer, "%s %d\n", rec.Host, rec.Pref)
		}
	default:
		return fmt.Errorf("unsupported record type: %s", typ)
	}

	return nil
}
