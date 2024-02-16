package dns

import (
	"github.com/isobit/ndog/internal"
)

var Scheme = &ndog.Scheme{
	Names:             []string{"dns"},
	Connect:           Connect,
	ConnectOptionHelp: connectOptionHelp,
	Listen:            Listen,
	ListenOptionHelp:  listenOptionHelp,

	Description: `
Connect performs a DNS request against the nameserver host and port specified in the URL.
The first path segment will be used as the request domain name.
The second path segment will be used as the request type, if specified. By default, "A" is used.
If no host is specified, the first entry in /etc/resolv.conf will be used.

Listen starts a basic UDP nameserver on the host and port specified in the URL.
By default, only response record data will be read from the response stream, and 
the response message will have the same type and name as the request.
The "zone" option allows answer resource records to be specified in the same format as and RFC 1035 zonefile;
when this is used, the default TTL is 0 and the default origin is set to the query domain name.

Examples:
	Query A records for example.net:
		ndog -c dns:///example.net
	Query AAAA records for example.net from 1.1.1.1:
		ndog -c dns://1.1.1.1/example.net/AAAA
	Serve "hello" as a response to requests on localhost, port 53000:
		ndog -l dns://localhost:53000 -d 'hello'
	Serve "hello" as a TXT response with domain name example.net and TTL of 10s:
		ndog -l dns://localhost:53000 -o zone -d 'example.net. 10 TXT hello'
	Serve "127.0.0.1" as an A response for any request domain name:
		ndog -l dns://localhost:53000 -o zone -d '@ A 127.0.0.1'
	`,
}
