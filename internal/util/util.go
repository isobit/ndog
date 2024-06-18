package util

import (
	"net/http"
	"sort"

	"github.com/isobit/ndog/internal/log"
)

func sortedHeaderKeys(header http.Header) []string {
	keys := make([]string, 0, len(header))
	for k := range header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func LogHeaders(prefix string, header http.Header) {
	for _, key := range sortedHeaderKeys(header) {
		values := header[key]
		for _, value := range values {
			log.Logf(1, "%s%s: %s", prefix, key, value)
		}
	}
}
