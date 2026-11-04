package mcp

import (
	"campusclaw/backend/internal/config"
	"campusclaw/backend/internal/db"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("MCP_INTEGRATION") != "1" {
		t.Skip("requires Compose MySQL and MCP_INTEGRATION=1")
	}
	cfg, e := config.Load()
	if e != nil {
		t.Fatal(e)
	}
	d, e := db.Open(cfg)
	if e != nil {
		t.Fatal(e)
	}
	if e = d.Ping(); e != nil {
		t.Fatal(e)
	}
	if e = Migrate(d); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { d.Close() })
	return d
}
func TestManagerIntegration(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()
	description := "安全教学提示"
	bad := false
	token := "unit-test-token-not-deployment"
	calls := 0
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			w.WriteHeader(401)
			return
		}
		var p struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
		}
		json.NewDecoder(r.Body).Decode(&p)
		var result any
		switch p.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": Protocol, "capabilities": map[string]any{"tools": map[string]any{}}}
		case "notifications/initialized":
			w.WriteHeader(202)
			return
		case "tools/list":
			if bad {
				w.WriteHeader(500)
				return
			}
			result = map[string]any{"tools": []any{map[string]any{"name": "formula_hint", "description": description, "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}, "required": []string{"query"}}, "annotations": map[string]any{"readOnlyHint": true, "openWorldHint": false}}}}
		default:
			calls++
		}
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": p.ID, "result": result})
	}))
	defer remote.Close()
	m, e := New(d, strings.Repeat("01", 32), token, remote.URL)
	if e != nil {
		t.Fatal(e)
	}
	name := fmt.Sprintf("it-%d", time.Now().UnixNano())
	s, e := m.Save(ctx, 1, 0, Input{name, remote.URL, token})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { m.Delete(ctx, 1, s.ID); d.Exec(`DELETE FROM mcp_tool_changes WHERE server_id=?`, s.ID) })
	data, _ := json.Marshal(s)
	if strings.Contains(string(data), token) || !s.HasAuth {
		t.Fatal("token leak")
	}
	var cipher []byte
	if e = d.QueryRow(`SELECT token_cipher FROM mcp_servers WHERE id=?`, s.ID).Scan(&cipher); e != nil || strings.Contains(string(cipher), token) {
		t.Fatal("plaintext database token")
	}
	if _, e = m.Get(ctx, 2, s.ID); e != sql.ErrNoRows {
		t.Fatal("cross class", e)
	}
	if _, e = m.Save(ctx, 1, 0, Input{name, remote.URL, token}); e == nil {
		t.Fatal("duplicate")
	}
	if _, e = m.Save(ctx, 1, 0, Input{"another", "http://169.254.169.254", token}); e == nil {
		t.Fatal("SSRF accepted")
	}
	for i := 0; i < 2; i++ {
		s, e = m.Connect(ctx, 1, s.ID)
		if e != nil || !s.Connected || s.ToolCount != 1 {
			t.Fatal(s, e)
		}
	}
	tools, e := m.Tools(ctx, 1, s.ID)
	if e != nil || tools[0].Approved {
		t.Fatal(tools, e)
	}
	ref := "formula_hint@" + name
	if m.ValidateRefs(ctx, 1, []string{ref}) == nil {
		t.Fatal("unapproved binding")
	}
	if e = m.Approve(ctx, 1, s.ID, "formula_hint", tools[0].Fingerprint); e != nil {
		t.Fatal(e)
	}
	if e = m.ValidateRefs(ctx, 1, []string{ref}); e != nil {
		t.Fatal(e)
	}
	oldHash := tools[0].Fingerprint
	description = "变更后的安全教学说明"
	if _, e = m.Connect(ctx, 1, s.ID); e != nil {
		t.Fatal(e)
	}
	tools, _ = m.Tools(ctx, 1, s.ID)
	if tools[0].Approved || m.Approve(ctx, 1, s.ID, "formula_hint", oldHash) == nil {
		t.Fatal("changed tool remained approved")
	}
	var n int
	d.QueryRow(`SELECT COUNT(*) FROM mcp_tool_changes WHERE server_id=?`, s.ID).Scan(&n)
	if n != 1 {
		t.Fatal(n)
	}
	if e = m.Disconnect(ctx, 1, s.ID); e != nil {
		t.Fatal(e)
	}
	tools, _ = m.Tools(ctx, 1, s.ID)
	if len(tools) != 0 || m.ValidateRefs(ctx, 1, []string{ref}) == nil {
		t.Fatal("disconnected cache/binding")
	}
	bad = true
	if _, e = m.Connect(ctx, 1, s.ID); e == nil {
		t.Fatal("failed discovery succeeded")
	}
	s, _ = m.Get(ctx, 1, s.ID)
	if s.Connected || s.ToolCount != 0 || s.LastError == "" {
		t.Fatal(s)
	}
	bad = false
	s, e = m.Save(ctx, 1, s.ID, Input{name, remote.URL, ""})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.Connect(ctx, 1, s.ID); e != nil {
		t.Fatal("blank edit did not preserve token", e)
	}
	if calls != 0 {
		t.Fatal("unexpected execution")
	}
}
