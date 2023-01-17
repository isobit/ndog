# ndog

Ndog is like [Ncat](https://nmap.org/ncat/), but friendlier! It is a networking
multitool that can act as a client or server for a number of network
schemes/protocols.

:warning: This project is under active development and is subject to major
breaking changes prior to the 1.0 release.

Features/goals (unchecked boxes have not been implemented yet):

- Composability with other UNIX-philosophy tools
	- [x] Streams multiplexed over STDIN/STDOUT
	- [x] Spawn subprocesses to handle individual streams (`--exec`)
- [x] Listening (act like a server)
- [x] Connecting (act like a client)
- [ ] Proxying (bridge listen and connect)
- Support for many more schemes/protocols than just TCP and UDP
	- [x] TCP
	- [x] UDP
	- [x] HTTP
	- [x] WebSockets
	- [x] PostgreSQL
	- [ ] SSH
	- [ ] QUIC
	- [ ] GraphQL
	- More schemes TBD!
- [ ] Support for TLS
- [ ] Interactive terminal user interface
	- [ ] Autocomplete per scheme/protocol
- [ ] Recording and playback with pattern matching

## Concepts

Ndog represents all schemes using _streams_, which are bidirectional (duplex)
byte streams. Streams can be multiplexed together (e.g. when listening with no
`--exec`), or handled individually (e.g. by an `--exec` subprocess).

| Context | Read | Write |
| --- | --- | --- |
| Listen | Data from client (request) | Data to client (response) |
| Connect | Data from server (response) | Data to server (request) |

## Schemes

| Scheme                   | Stream per            | Default representation |
| ---                      | ---                   | ----                   |
| `tcp`                    | TCP connection        | Raw data               |
| `udp`                    | UDP remote address    | Raw data               |
| `ws`                     | WebSocket             | Raw data               |
| `http`                   | HTTP request          | Request/response body  |
| `postgresql`, `postgres` | PostgreSQL connection | SQL statements/row CSV |

## Examples

| Description                                          | Incantation                                     |
| ---                                                  | ---                                             |
| Start a TCP server on port 8000, all interfaces      | `ndog -l tcp://:8000`                           |
| Start a UDP server on port 8125, localhost           | `ndog -l udp://localhost:8125`                  |
| Start an HTTP server on port 8080, all interfaces    | `ndog -l http://:8080`                          |
| Serve current directory file system over HTTP server | `ndog -l http://localhost:8080 -o serve_file=.` |
| Connect to a TCP server on port 8000, localhost      | `ndog -c tcp://localhost:8000`                  |
| Connect to a UDP server on port 8125, localhost      | `ndog -c udp://localhost:8000`                  |
