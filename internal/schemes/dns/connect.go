package dns

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"reflect"
	"strings"

	"github.com/isobit/ndog/internal"
	"github.com/isobit/ndog/internal/log"

	"github.com/miekg/dns"
)

var connectOptionHelp = ndog.OptionsHelp{}.
	Add("norecurse", "", "disable recursion").
	Add("json", "", "use JSON representation for answers")

type connectOptions struct {
	NoRecurse bool
	JSON      bool
}

func extractConnectOptions(opts ndog.Options) (connectOptions, error) {
	o := connectOptions{}

	if _, ok := opts.Pop("norecurse"); ok {
		o.NoRecurse = true
	}

	if _, ok := opts.Pop("json"); ok {
		o.JSON = true
	}

	return o, opts.Done()
}

func Connect(cfg ndog.ConnectConfig) error {
	opts, err := extractConnectOptions(cfg.Options)
	if err != nil {
		return err
	}

	nameserver := cfg.URL.Host
	if nameserver == "" {
		conf, err := dns.ClientConfigFromFile("/etc/resolv.conf")
		if err != nil {
			return fmt.Errorf("failed to determine nameserver from /etc/resolv.conf: %w", err)
		}
		nameserver = conf.Servers[0]
		log.Logf(10, "using first nameserver from /etc/resolv.conf: %s", nameserver)
	}
	nsHost, nsPort, err := net.SplitHostPort(nameserver)
	if err != nil {
		if addrErr, ok := err.(*net.AddrError); !ok || addrErr.Err != "missing port in address" {
			return fmt.Errorf("error splitting nameserver into host and port: %w", err)
		}
		nsHost = nameserver
		nsPort = "53"
	}
	nameserver = net.JoinHostPort(nsHost, nsPort)

	path := strings.SplitN(strings.TrimPrefix(cfg.URL.Path, "/"), "/", 2)
	qname := path[0]

	typ := "A"
	if len(path) == 2 {
		typ = path[1]
	}

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
			Name:   dns.Fqdn(qname),
			Qtype:  qtype,
			Qclass: dns.ClassINET,
		}},
	}

	log.Logf(1, "query:")
	log.Logf(1, req.String())

	res, _, err := client.Exchange(req, nameserver)
	if err != nil {
		return err
	}

	log.Logf(1, "answer:")
	log.Logf(1, res.String())

	if res.Rcode != dns.RcodeSuccess {
		return fmt.Errorf("received unsuccessful response code: %s", dns.RcodeToString[res.Rcode])
	}

	if opts.JSON {
		data, err := json.Marshal(res)
		if err != nil {
			return err
		}
		cfg.Stream.Writer.Write(data)
		io.WriteString(cfg.Stream.Writer, "\n")
		return nil
	}

	for _, answer := range res.Answer {
		hdr := answer.Header()
		if hdr.Rrtype != qtype {
			log.Logf(10, "skipping answer type %s (does not match %s)", dns.TypeToString[hdr.Rrtype], dns.TypeToString[qtype])
			continue
		}
		switch ans := answer.(type) {
		case *dns.A:
			fmt.Fprintln(cfg.Stream.Writer, ans.A)
		case *dns.AAAA:
			fmt.Fprintln(cfg.Stream.Writer, ans.AAAA)
		case *dns.CNAME:
			fmt.Fprintln(cfg.Stream.Writer, ans.Target)
		case *dns.TXT:
			for _, txt := range ans.Txt {
				fmt.Fprintln(cfg.Stream.Writer, txt)
			}
		case *dns.MX:
			fmt.Fprintln(cfg.Stream.Writer, ans.Mx)
		case *dns.NS:
			fmt.Fprintln(cfg.Stream.Writer, ans.Ns)
		default:
			return fmt.Errorf("unsupported answer type %s: %s", reflect.TypeOf(ans), ans)
		}
	}
	return nil
}
