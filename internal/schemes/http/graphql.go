package http

type GraphQLRequest struct {
	Query         string `json:"query"`
	OperationName string `json:"operationName,omitempty"`
	Variables     any    `json:"Variables,omitempty"`
}

type GraphQLResponse struct {
	Data   any   `json:"data"`
	Errors []any `json:"errors"`
}
