package auth

import (
	"net/http/httptest"
	"testing"
)

func TestWriteOrigin(t *testing.T) {
	for _, v := range []struct {
		origin, site string
		allow        bool
	}{{"", "", true}, {"http://example.com", "same-origin", true}, {"https://evil.example", "cross-site", false}, {"null", "", false}, {"http://example.com/path", "", false}, {"http://example.com", "cross-site", false}} {
		r := httptest.NewRequest("POST", "http://example.com/api/agents", nil)
		if v.origin != "" {
			r.Header.Set("Origin", v.origin)
		}
		r.Header.Set("Sec-Fetch-Site", v.site)
		if sameOriginWrite(r) != v.allow {
			t.Fatalf("unexpected decision for %v", v)
		}
	}
}
