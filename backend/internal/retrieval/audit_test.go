package retrieval

import (
	"campusclaw/backend/internal/observability"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// H2: normal requests collect real usage without enabling the private trace.
func TestOrdinaryCallAuditWithoutTrace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Content-Type"), "json") {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}}`))
		}
	}))
	defer server.Close()
	a := observability.New()
	ctx := observability.With(context.Background(), a)
	observability.Bind(ctx, 1, nil, "test")
	c := &ChatClient{BaseURL: server.URL, Model: "test"}
	if _, e := c.DecideTools(ctx, nil, nil); e != nil {
		t.Fatal(e)
	}
	if a.Fields()["usage"].(map[string]any)["total_tokens"] != int64(10) {
		t.Fatal(a.Fields())
	}
	stream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"content-marker\"}}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":11,\"completion_tokens\":4,\"total_tokens\":15}}\n\ndata: [DONE]\n\n"))
	}))
	defer stream.Close()
	c.BaseURL = stream.URL
	if e := c.Stream(ctx, nil, func(string) error { return nil }); e != nil {
		t.Fatal(e)
	}
	u := a.Fields()["usage"].(map[string]any)
	if u["total_tokens"] != int64(25) || u["reported_calls"] != 2 || u["complete"] != true {
		t.Fatal(a.Fields())
	}
}
