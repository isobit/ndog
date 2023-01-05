package schemes

import (
	"fmt"

	"github.com/isobit/ndog"
	"github.com/isobit/ndog/schemes/http"
	"github.com/isobit/ndog/schemes/postgresql"
	"github.com/isobit/ndog/schemes/tcp"
	"github.com/isobit/ndog/schemes/udp"
	"github.com/isobit/ndog/schemes/websocket"
)

var Registry = map[string]*ndog.Scheme{}

func registerScheme(scheme *ndog.Scheme) {
	for _, name := range scheme.Names {
		if _, exists := Registry[name]; exists {
			panic(fmt.Sprintf("conflicting scheme name: %s", name))
		}
		Registry[name] = scheme
	}
}

func init() {
	registerScheme(http.HTTPSScheme)
	registerScheme(http.HTTPScheme)
	registerScheme(postgresql.Scheme)
	registerScheme(tcp.Scheme)
	registerScheme(udp.Scheme)
	registerScheme(websocket.Scheme)
}
