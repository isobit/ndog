package schemes

import (
	"fmt"

	"github.com/isobit/ndog/internal"
	"github.com/isobit/ndog/internal/schemes/http"
	"github.com/isobit/ndog/internal/schemes/postgresql"
	"github.com/isobit/ndog/internal/schemes/ssh"
	"github.com/isobit/ndog/internal/schemes/tcp"
	"github.com/isobit/ndog/internal/schemes/udp"
	"github.com/isobit/ndog/internal/schemes/websocket"
)

func init() {
	registerSchemes(
		http.HTTPScheme,
		http.HTTPSScheme,
		http.HTTPGraphQLScheme,
		postgresql.Scheme,
		postgresql.ListenScheme,
		postgresql.NotifyScheme,
		ssh.Scheme,
		tcp.Scheme,
		udp.Scheme,
		websocket.WSScheme,
		websocket.WSSScheme,
	)
}

var Registry = map[string]*ndog.Scheme{}

func registerSchemes(schemes ...*ndog.Scheme) {
	for _, scheme := range schemes {
		for _, name := range scheme.Names {
			if _, exists := Registry[name]; exists {
				panic(fmt.Sprintf("conflicting scheme name: %s", name))
			}
			Registry[name] = scheme
		}
	}
}

func Lookup(urlScheme string) *ndog.Scheme {
	return Registry[urlScheme]
}
