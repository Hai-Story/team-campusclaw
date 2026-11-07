package retrieval

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestTraceActualHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var v map[string]any
		json.NewDecoder(r.Body).Decode(&v)
		if r.Header.Get("Authorization") != "Bearer secret-key" {
			t.Error("missing outbound auth")
		}
		if v["stream"] == true {
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"真实片段\"}}]}\n\ndata: [DONE]\n\n")
		} else {
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call1","type":"function","function":{"name":"mcp_1_hint","arguments":"{}"}}]}}]}`)
		}
	}))
	defer server.Close()
	c := ChatClient{BaseURL: server.URL, APIKey: "secret-key", Model: "fixture-model"}
	trace := NewTrace(c.APIKey)
	ctx := WithTrace(context.Background(), trace)
	messages := []ChatMessage{{Role: "system", Content: "TEMPLATE"}, {Role: "user", Content: "question"}}
	defs := []ToolDefinition{{Type: "function", Function: ToolFunction{Name: "mcp_1_hint", Parameters: json.RawMessage(`{"type":"object"}`)}}}
	if _, err := c.DecideTools(ctx, messages, defs); err != nil {
		t.Fatal(err)
	}
	answer := ""
	if err := c.Stream(ctx, messages, func(s string) error { answer += s; return nil }); err != nil {
		t.Fatal(err)
	}
	if answer != "真实片段" || len(trace.Calls) != 2 || trace.Calls[1].OutputText != "真实片段" {
		t.Fatal(answer, len(trace.Calls))
	}
	var req map[string]any
	json.Unmarshal(trace.Calls[0].Request, &req)
	if req["tool_choice"] != "auto" || req["stream"] != false || req["max_tokens"] != float64(700) {
		t.Fatal(req)
	}
	json.Unmarshal(trace.Calls[1].Request, &req)
	if req["stream"] != true || req["max_tokens"] != float64(500) || !strings.Contains(trace.Calls[1].Response, "[DONE]") {
		t.Fatal(req)
	}
	if strings.Contains(string(trace.JSON()), "secret-key") || strings.Contains(string(trace.JSON()), "Authorization") {
		t.Fatal("credential captured")
	}
	if err := c.Stream(context.Background(), messages, func(string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if len(trace.Calls) != 2 {
		t.Fatal("capture leaked")
	}
}
func TestTraceBoundsAndConcurrentIsolation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, strings.Repeat("X", traceLimit+10))
	}))
	defer server.Close()
	c := ChatClient{BaseURL: server.URL}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			trace := NewTrace()
			ctx := WithTrace(context.Background(), trace)
			for j := 0; j < 9; j++ {
				c.Stream(ctx, []ChatMessage{{Role: "user", Content: "private"}}, func(string) error { return nil })
			}
			if len(trace.Calls) != 8 || !trace.Truncated || !trace.Calls[0].ResponseTruncated || len(trace.Calls[0].Response) != traceLimit {
				t.Error("unbounded trace")
			}
		}()
	}
	wg.Wait()
}
