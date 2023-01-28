package schemes

import (
	"fmt"

	"github.com/isobit/ndog"
	"github.com/isobit/ndog/schemes/http"
	"github.com/isobit/ndog/schemes/postgresql"
	"github.com/isobit/ndog/schemes/ssh"
	"github.com/isobit/ndog/schemes/tcp"
	"github.com/isobit/ndog/schemes/udp"
	"github.com/isobit/ndog/schemes/websocket"
)

func init() {
	registerSchemes(
		http.HTTPSScheme,
		http.HTTPScheme,
		postgresql.Scheme,
		ssh.Scheme,
		tcp.Scheme,
		udp.Scheme,
		websocket.Scheme,
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
