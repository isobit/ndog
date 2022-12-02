# ndog

Ndog is like [Ncat](https://nmap.org/ncat/), but friendlier!

:warning: This project is under active development and is subject to major
breaking changes prior to the 1.0 release.

## Examples

| Description                                       | Incantation                    |
| ---                                               | ---                            |
| Start a TCP server on port 8000, all interfaces   | `ndog -l tcp://:8000`          |
| Start a UDP server on port 8125, localhost        | `ndog -l udp://localhost:8125` |
| Start an HTTP server on port 8080, all interfaces | `ndog -l http://:8080`         |
| Connect to a TCP server on port 8000, localhost   | `ndog -c tcp://localhost:8000` |
| Connect to a UDP server on port 8125, localhost   | `ndog -c udp://localhost:8000` |
