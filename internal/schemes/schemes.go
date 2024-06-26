package schemes

import (
	"fmt"

	"github.com/isobit/ndog/internal"
	"github.com/isobit/ndog/internal/schemes/dns"
	"github.com/isobit/ndog/internal/schemes/http"
	"github.com/isobit/ndog/internal/schemes/postgresql"
	"github.com/isobit/ndog/internal/schemes/ssh"
	"github.com/isobit/ndog/internal/schemes/tcp"
	"github.com/isobit/ndog/internal/schemes/udp"
	"github.com/isobit/ndog/internal/schemes/websocket"
)

func init() {
	registerSchemes(
		dns.Scheme,
		http.HTTPScheme,
		http.HTTPGraphQLScheme,
		postgresql.Scheme,
		postgresql.ListenScheme,
		postgresql.NotifyScheme,
		ssh.Scheme,
		tcp.Scheme,
		tcp.TLSScheme,
		udp.Scheme,
		websocket.WSScheme,
	)
}

var Registry = map[string]*ndog.Scheme{}
var fullRegistry = map[string]*ndog.Scheme{}

func registerSchemes(schemes ...*ndog.Scheme) {
	for _, scheme := range schemes {
		for _, name := range scheme.Names {
			if _, exists := fullRegistry[name]; exists {
				panic(fmt.Sprintf("conflicting scheme name: %s", name))
			}
			fullRegistry[name] = scheme
			Registry[name] = scheme
		}
		for _, name := range scheme.HiddenNames {
			if _, exists := fullRegistry[name]; exists {
				panic(fmt.Sprintf("conflicting scheme name: %s", name))
			}
			fullRegistry[name] = scheme
		}
	}
}

func Lookup(urlScheme string) (*ndog.Scheme, bool) {
	scheme, ok := fullRegistry[urlScheme]
	return scheme, ok
}
