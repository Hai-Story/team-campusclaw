package auth

import (
	"net/http"
	"net/url"
	"strings"
)

// Bearer requests have no ambient credential. Reject authenticated browser writes
// from a different origin as defense in depth; clients without Origin remain valid.
func sameOriginWrite(r *http.Request) bool {
	if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
		return true
	}
	if strings.EqualFold(r.Header.Get("Sec-Fetch-Site"), "cross-site") {
		return false
	}
	origins, exists := r.Header["Origin"]
	if !exists {
		return true
	}
	if len(origins) != 1 {
		return false
	}
	u, err := url.Parse(origins[0])
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || u.Host == "" {
		return false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	// Nginx terminates TLS and overwrites this header in the single trusted ingress.
	if proto := r.Header.Get("X-Forwarded-Proto"); proto == "https" {
		scheme = proto
	}
	return u.Scheme == scheme && strings.EqualFold(u.Host, r.Host)
}
