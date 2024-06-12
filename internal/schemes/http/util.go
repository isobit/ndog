package http

import (
	"net/http"
	"sort"
)

func sortedHeaderKeys(header http.Header) []string {
	keys := make([]string, 0, len(header))
	for k := range header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
