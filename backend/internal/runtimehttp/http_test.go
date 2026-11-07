package runtimehttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// team-runtime-foundation S8/S9: generated IDs ignore client values and survive SSE.
func TestRequestIDAndStreaming(t *testing.T) {
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ID(r.Context()) == "attacker-controlled" {
			t.Error("trusted external ID")
		}
		w.(http.Flusher).Flush()
		w.Write([]byte("stream"))
	}))
	ids := map[string]bool{}
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test?secret=hidden", nil)
		req.Header.Set("X-Request-ID", "attacker-controlled")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		id := rec.Header().Get("X-Request-ID")
		if len(id) != 32 || ids[id] || !rec.Flushed {
			t.Fatal("invalid identity or missing flush")
		}
		ids[id] = true
	}
}

// S5/S6/S7: unavailable dependencies remain bounded and /live ignores dependencies.
func TestProbeTimeoutAndReadOnly(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer slow.Close()
	mux := http.NewServeMux()
	Probes(mux, nil, "test", slow.URL, slow.URL)
	handler := Middleware(mux)
	start := time.Now()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/health", nil))
	if rec.Code != 503 || time.Since(start) > 3*time.Second {
		t.Fatal("probe unbounded or incorrect status")
	}
	var out map[string]any
	if json.Unmarshal(rec.Body.Bytes(), &out) != nil || out["request_id"] != rec.Header().Get("X-Request-ID") {
		t.Fatal("trace mismatch")
	}
	for _, tc := range []struct {
		method, path string
		want         int
	}{{"GET", "/live", 200}, {"GET", "/health?detail=full", 400}, {"POST", "/health", 405}} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if rec.Code != tc.want {
			t.Fatalf("%s: %d", tc.path, rec.Code)
		}
	}
}
