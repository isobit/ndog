package dns

import (
	"context"
	// "errors"
	"fmt"
	// "io"
	"net"
	"reflect"
	"strings"

	"github.com/isobit/ndog/internal"
	"github.com/isobit/ndog/internal/log"

	"github.com/miekg/dns"
)

var Scheme = &ndog.Scheme{
	Names:             []string{"dns"},
	Connect:           Connect,
	ConnectOptionHelp: connectOptionHelp,
}

var connectOptionHelp = ndog.OptionsHelp{}.
	Add("norecurse", "", "disable recursion")

type connectOptions struct {
	NoRecurse bool
}

func extractConnectOptions(opts ndog.Options) (connectOptions, error) {
	o := connectOptions{}

	if _, ok := opts.Pop("norecurse"); ok {
		o.NoRecurse = true
	}

	return o, opts.Done()
}
func Connect(cfg ndog.ConnectConfig) error {
	ctx := context.TODO()

	opts, err := extractConnectOptions(cfg.Options)
	if err != nil {
		return err
	}

	// if cfg.URL.Host != "" {
	// 	return fmt.Errorf("specifying a nameserver is not supported yet")
	// }

	path := strings.SplitN(strings.TrimPrefix(cfg.URL.Path, "/"), "/", 2)

	host := path[0]

	typ := "A"
	if len(path) == 2 {
		typ = path[1]
	}

	nameserver := cfg.URL.Host
	if nameserver != "" {
		qtype, ok := dns.StringToType[strings.ToUpper(typ)]
		if !ok {
			return fmt.Errorf("unknown type: %s", typ)
		}

		client := &dns.Client{}
		req := &dns.Msg{
			MsgHdr: dns.MsgHdr{
				Id:               dns.Id(),
				Opcode:           dns.OpcodeQuery,
				RecursionDesired: !opts.NoRecurse,
			},
			Question: []dns.Question{{
				Name:   dns.Fqdn(host),
				Qtype:  qtype,
				Qclass: dns.ClassINET,
			}},
		}
		log.Logf(10, "query:")
		log.Logf(10, req.String())
		res, _, err := client.Exchange(req, nameserver)
		if err != nil {
			return err
		}
		log.Logf(10, "answer:")
		log.Logf(10, res.String())
		for _, answer := range res.Answer {
			switch ans := answer.(type) {
			case *dns.A:
				fmt.Fprintf(cfg.Stream.Writer, "%s\n", ans.A)
			case *dns.AAAA:
				fmt.Fprintf(cfg.Stream.Writer, "%s\n", ans.AAAA)
			case *dns.CNAME:
				fmt.Fprintf(cfg.Stream.Writer, "%s\n", ans.Target)
			default:
				return fmt.Errorf("unexpected answer type: %s", reflect.TypeOf(ans))
			}
			// r, ok := ans.(*dns.A)
			// if !ok {
			// }
			// fmt.Fprintf(cfg.Stream.Writer, "%s\n", r.A)
		}
		return nil
	}

	r := net.Resolver{}

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
