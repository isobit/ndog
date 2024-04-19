package dns

import (
	"encoding/json"
	"fmt"
	"io"
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
	path := strings.SplitN(strings.TrimPrefix(cfg.URL.Path, "/"), "/", 2)
	host := path[0]

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
			return fmt.Errorf("unexpected answer type: %s", reflect.TypeOf(ans))
		}
	}
	return nil
}
