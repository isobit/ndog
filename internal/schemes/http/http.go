package http

import (
	"net/http"
	"strings"

	"github.com/isobit/ndog/internal"
)

var HTTPScheme = &ndog.Scheme{
	Names:       []string{"http", "https"},
	HiddenNames: []string{}, // will be initialized in init()

	Connect: Connect,
	Listen:  Listen,

	Description: `
Connect sends input as an HTTP request body to the specified URL.

Listen sends input as an HTTP request body to the specified URL.

Examples:
	GET request: ndog -c 'http://example.net/' -d ''
	Echo server: ndog -l 'http://localhost:8080' -x 'cat'
	File server: ndog -l 'http://localhost:8080' -o 'serve_file=.'
	`,
	ConnectOptionHelp: connectOptionHelp,
	ListenOptionHelp:  listenOptionHelp,
}

func init() {
	for _, scheme := range HTTPScheme.Names {
		for _, method := range methods {
			HTTPScheme.HiddenNames = append(HTTPScheme.HiddenNames, scheme+"+"+method)
			HTTPScheme.HiddenNames = append(HTTPScheme.HiddenNames, scheme+"+"+strings.ToLower(method))
		}
	}
}

var HTTPGraphQLScheme = &ndog.Scheme{
	Names:   []string{"http+graphql", "https+graphql"},
	Connect: Connect,

	Description: `
Connect sends input as an GraphQL query to the specified URL over an HTTP POST request.

Examples:
	./ndog -c https+graphql://countries.trevorblades.com/graphql -d 'query { countries { name } }'
	`,
	ConnectOptionHelp: connectOptionHelpGraphql,
}

var methods = []string{
	http.MethodGet,
	http.MethodHead,
	http.MethodPost,
	http.MethodPut,
	http.MethodPatch,
	http.MethodDelete,
	http.MethodConnect,
	http.MethodOptions,
	http.MethodTrace,
}
