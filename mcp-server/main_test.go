package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProtocol(t *testing.T) {
	h := handler("test-token-only-not-a-deployment-secret")
	for _, tc := range []struct {
		body, auth string
		status     int
		key        string
	}{
		{`{"jsonrpc":"2.0","id":7,"method":"tools/list"}`, "", 401, "error"},
		{`{"jsonrpc":"2.0","id":7,"method":"tools/list"}`, "bad", 401, "error"},
		{`{"jsonrpc":"2.0","id":7,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`, "test-token-only-not-a-deployment-secret", 200, "result"},
		{`{"jsonrpc":"2.0","id":7,"method":"tools/list"}`, "test-token-only-not-a-deployment-secret", 200, "result"},
		{`{"jsonrpc":"2.0","method":"notifications/initialized"}`, "test-token-only-not-a-deployment-secret", 202, ""},
		{`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"formula_hint","arguments":{"query":"去括号"}}}`, "test-token-only-not-a-deployment-secret", 200, "result"},
		{`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"unit_convert","arguments":{"value":5,"from":"m","to":"cm"}}}`, "test-token-only-not-a-deployment-secret", 200, "result"},
		{`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"unit_convert","arguments":{"value":5,"from":"m","to":"g"}}}`, "test-token-only-not-a-deployment-secret", 200, "error"},
		{`{"jsonrpc":"2.0","id":7,"method":"not_a_method"}`, "test-token-only-not-a-deployment-secret", 200, "error"},
		{`not JSON`, "test-token-only-not-a-deployment-secret", 200, "error"},
	} {
		r := httptest.NewRequest("POST", "/mcp", strings.NewReader(tc.body))
		if tc.auth != "" {
			r.Header.Set("Authorization", "Bearer "+tc.auth)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatal(w.Code, w.Body.String())
		}
		if tc.key != "" {
			var v map[string]any
			if json.Unmarshal(w.Body.Bytes(), &v) != nil || v[tc.key] == nil {
				t.Fatal(w.Body.String())
			}
			if tc.key == "result" && v["id"] != float64(7) {
				t.Fatal("id mismatch")
			}
		}
	}
	r := httptest.NewRequest("GET", "/mcp", nil)
	r.Header.Set("Authorization", "Bearer test-token-only-not-a-deployment-secret")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatal(w.Code)
	}
}
func TestHintsOnly(t *testing.T) {
	text, err := execute("unit_convert", json.RawMessage(`{"value":5,"from":"m","to":"cm"}`))
	if err != nil || !strings.Contains(text, "100") || strings.Contains(text, "500") {
		t.Fatal(text, err)
	}
	if _, err = execute("formula_hint", json.RawMessage(`{"query":"问题","secret":"x"}`)); err == nil {
		t.Fatal("accepted excess arguments")
	}
}
