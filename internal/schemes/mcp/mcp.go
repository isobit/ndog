package mcp

import (
	ndog "github.com/isobit/ndog/internal"
)

var Scheme = &ndog.Scheme{
	Names:   []string{"mcp", "mcp+http", "mcp+https"},
	Connect: Connect,

	Description: `
Connect to an MCP (Model Context Protocol) server over HTTP.

The URL path is the MCP server endpoint. The fragment (#) specifies the operation:
  tools/list         - list available tools
  tools/call/<name>  - call a tool
  resources/list     - list available resources
  resources/read/<uri> - read a resource
  prompts/list       - list available prompts
  prompts/get/<name> - get a prompt
  <name>             - call a tool (shorthand)

If no fragment is specified, defaults to tools/list.

Examples:
  List tools:    ndog -c 'mcp://localhost:8080/mcp#tools/list' -d ''
  Call a tool:   ndog -c 'mcp://localhost:8080/mcp#greet' -o param.name=world -d ''
  Read resource: ndog -c 'mcp://localhost:8080/mcp#resources/read/file:///etc/hosts' -d ''
	`,

	ConnectOptionHelp: ndog.OptionsHelp{}.
		Add("header.<name>", "<value>", "extra HTTP headers to send (e.g. header.Authorization=Bearer token)").
		Add("param.<key>", "<value>", "tool call argument").
		Add("json", "", "output full JSON response instead of text").
		Add("timeout", "<duration>", "operation timeout"),
}
