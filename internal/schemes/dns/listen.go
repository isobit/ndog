package dns

import (
	"bufio"
	"fmt"

	"github.com/isobit/ndog/internal"
	"github.com/isobit/ndog/internal/log"

	"github.com/miekg/dns"
)

var listenOptionHelp = ndog.OptionsHelp{}.
	Add("zone", "", "parse response stream as an RFC 1035 zonefile")

type listenOptions struct {
	Zone bool
}

func extractListenOptions(opts ndog.Options) (listenOptions, error) {
	o := listenOptions{}

	if _, ok := opts.Pop("zone"); ok {
		o.Zone = true
	}

	return o, opts.Done()
}

func dnsHandler(f func(*dns.Msg) (dns.RR, error)) dns.Handler {
	return dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		m := &dns.Msg{}
		m.SetReply(r)

		rr, err := f(r)
		if err != nil {
			log.Logf(-1, "%s", err)
			m.Rcode = dns.RcodeServerFailure
		}
		if rr != nil {
			m.Answer = []dns.RR{rr}
		}
		if err := w.WriteMsg(m); err != nil {
			log.Logf(-1, "error writing response: %s", err)
		}
	})
}

func Listen(cfg ndog.ListenConfig) error {
	opts, err := extractListenOptions(cfg.Options)
	if err != nil {
		return err
	}

	s := dns.Server{
		Net:  "udp",
		Addr: cfg.URL.Host,
		Handler: dnsHandler(func(r *dns.Msg) (dns.RR, error) {
			if len(r.Question) != 1 {
				return nil, fmt.Errorf("expected 1 question but got %d", len(r.Question))
			}
			q := r.Question[0]

			stream := cfg.StreamManager.NewStream(fmt.Sprintf("%d", r.Id))
			defer stream.Close()

			fmt.Fprintf(
				stream.Writer,
				"%s %s %s\n",
				q.Name,
				dns.ClassToString[q.Qclass],
				dns.TypeToString[q.Qtype],
			)
			stream.Writer.Close()

			if opts.Zone {
				zp := dns.NewZoneParser(stream.Reader, q.Name, "")
				zp.SetDefaultTTL(0)
				rr, _ := zp.Next()
				if err := zp.Err(); err != nil {
					return nil, fmt.Errorf("error parsing zone from stream: %w", err)
				}
				return rr, nil
			} else {
				values := []string{}
				scanner := bufio.NewScanner(stream.Reader)
				for scanner.Scan() {
					values = append(values, scanner.Text())
				}
				if err := scanner.Err(); err != nil {
					return nil, fmt.Errorf("error scanning stream reader: %w", err)
				}

				hdr := dns.RR_Header{
					Name:   q.Name,
					Rrtype: q.Qtype,
					Class:  q.Qclass,
					Ttl:    0,
				}
				var rr dns.RR
				switch q.Qtype {
				case dns.TypeTXT:
					rr = &dns.TXT{
						Hdr: hdr,
						Txt: values,
					}
				// TODO additional types
				// case dns.TypeA:
				// 	rr = &dns.A{
				// 		Hdr: hdr,
				// 		A: values,
				// 	}
				default:
					return nil, fmt.Errorf("unsupported query type: %s", dns.TypeToString[q.Qtype])
				}
				return rr, nil
			}
		}),
	}
	log.Logf(0, "listening: %s", s.Addr)
	return s.ListenAndServe()
}
