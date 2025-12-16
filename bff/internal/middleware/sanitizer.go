package middleware

import (
	"net/http"
	"strings"
)

// HeaderSanitizer removes internal headers from incoming requests.
type HeaderSanitizer struct {
	headersToRemove map[string]struct{}
}

// NewHeaderSanitizer creates a new header sanitizer.
func NewHeaderSanitizer(headers []string) *HeaderSanitizer {
	headerMap := make(map[string]struct{}, len(headers))
	for _, h := range headers {
		// Store in canonical form (lowercase)
		headerMap[strings.ToLower(h)] = struct{}{}
	}

	return &HeaderSanitizer{
		headersToRemove: headerMap,
	}
}

// Middleware returns an HTTP middleware that sanitizes headers.
func (s *HeaderSanitizer) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Remove internal headers
		for header := range r.Header {
			if _, shouldRemove := s.headersToRemove[strings.ToLower(header)]; shouldRemove {
				r.Header.Del(header)
			}
		}

		next.ServeHTTP(w, r)
	})
}
