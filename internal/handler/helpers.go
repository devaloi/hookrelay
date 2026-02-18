package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/devaloi/hookrelay/internal/domain"
)

// requireMethod checks that the request uses the expected HTTP method.
// Returns true if the method matches; writes a 405 response and returns false otherwise.
func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

// parsePagination extracts limit and offset query parameters with validation.
// Returns an error if the values are not valid integers.
func parsePagination(r *http.Request) (limit, offset int, err error) {
	limitStr := r.URL.Query().Get("limit")
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid limit parameter: %w", err)
		}
	}
	if limit <= 0 {
		limit = domain.DefaultPaginationLimit
	}

	offsetStr := r.URL.Query().Get("offset")
	if offsetStr != "" {
		offset, err = strconv.Atoi(offsetStr)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid offset parameter: %w", err)
		}
	}

	return limit, offset, nil
}

// sensitiveHeaders lists headers that should be filtered before storage.
var sensitiveHeaders = map[string]bool{
	"Authorization": true,
	"Cookie":        true,
	"Set-Cookie":    true,
}

// filterHeaders removes sensitive headers (case-insensitive match).
func filterHeaders(headers map[string]string) map[string]string {
	filtered := make(map[string]string, len(headers))
	for key, value := range headers {
		if sensitiveHeaders[http.CanonicalHeaderKey(key)] {
			continue
		}
		filtered[key] = value
	}
	return filtered
}

// isAllowedScheme checks whether a URL uses an allowed scheme (http or https).
func isAllowedScheme(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}
