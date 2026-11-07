package tutor

import (
	"campusclaw/backend/internal/auth"
	"campusclaw/backend/internal/retrieval"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTraceOwnershipIntegration(t *testing.T) {
	d, cfg := integrationDB(t)
	if err := Migrate(d); err != nil {
		t.Fatal(err)
	}
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"验收回答\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer model.Close()
	s := NewService(d, &retrieval.Searcher{DB: d, Cfg: cfg}, &retrieval.ChatClient{BaseURL: model.URL, APIKey: "fixture-secret", Model: "fixture"})
	a := auth.NewService(d, cfg)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/login", a.Login)
	mux.HandleFunc("POST /api/conversations/{id}/messages", a.Middleware(auth.RequireStudent(s.SendMessage)))
	mux.HandleFunc("GET /api/conversations/{id}/messages/{message}/trace", a.Middleware(auth.RequireStudent(s.MessageTrace)))
	server := httptest.NewServer(mux)
	defer server.Close()
	tag := fmt.Sprint(time.Now().UnixNano())
	res, err := d.Exec(`INSERT INTO users(username,password_hash,role,class_id) SELECT ?,password_hash,'student',1 FROM users WHERE username='student_a1'`, "trace-peer"+tag)
	if err != nil {
		t.Fatal(err)
	}
	peer, _ := res.LastInsertId()
	defer d.Exec(`DELETE FROM users WHERE id=?`, peer)
	tokens := map[string]string{}
	for _, u := range []string{"student_a1", "student_b1", "teacher_a", "trace-peer" + tag} {
		pass := cfg.SeedStudentPassword
		if u == "teacher_a" {
			pass = cfg.SeedTeacherPassword
		}
		r, _ := http.Post(server.URL+"/api/login", "application/json", strings.NewReader(fmt.Sprintf(`{"username":%q,"password":%q}`, u, pass)))
		var v map[string]any
		json.NewDecoder(r.Body).Decode(&v)
		r.Body.Close()
		if r.StatusCode != 200 {
			t.Fatal("login")
		}
		tokens[u] = v["token"].(string)
	}
	defer func() {
		for _, token := range tokens {
			d.Exec(`DELETE FROM sessions WHERE token_hash=SHA2(?,256)`, token)
		}
	}()
	res, err = d.Exec(`INSERT INTO tutor_agents(class_id,name,prompt) VALUES(1,?,'TRACE_TEMPLATE')`, "trace-agent"+tag)
	if err != nil {
		t.Fatal(err)
	}
	aid, _ := res.LastInsertId()
	defer d.Exec(`DELETE FROM tutor_agents WHERE id=?`, aid)
	var uid int64
	d.QueryRow(`SELECT id FROM users WHERE username='student_a1'`).Scan(&uid)
	res, err = d.Exec(`INSERT INTO conversations(class_id,user_id,agent_id) VALUES(1,?,?)`, uid, aid)
	if err != nil {
		t.Fatal(err)
	}
	cid, _ := res.LastInsertId()
	defer d.Exec(`DELETE FROM conversations WHERE id=?`, cid)
	call := func(who, method, path, body string) (int, string) {
		r, _ := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+tokens[who])
		r.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	status, b := call("student_a1", "POST", fmt.Sprintf("/api/conversations/%d/messages", cid), `{"content":"第一次提问","capture_trace":true}`)
	if status != 200 || !strings.Contains(b, "event: done") {
		t.Fatal(status, b)
	}
	var mid int64
	d.QueryRow(`SELECT id FROM messages WHERE conversation_id=? AND role='assistant' ORDER BY id DESC LIMIT 1`, cid).Scan(&mid)
	p := fmt.Sprintf("/api/conversations/%d/messages/%d/trace", cid, mid)
	status, b = call("student_a1", "GET", p, "")
	if status != 200 || !strings.Contains(b, "TRACE_TEMPLATE") || strings.Contains(b, "fixture-secret") {
		t.Fatal(status, b)
	}
	for who, want := range map[string]int{"teacher_a": 403, "student_b1": 404, "trace-peer" + tag: 404} {
		status, _ = call(who, "GET", p, "")
		if status != want {
			t.Fatal(who, status)
		}
	}
	status, b = call("student_a1", "POST", fmt.Sprintf("/api/conversations/%d/messages", cid), `{"content":"默认不记录"}`)
	if status != 200 {
		t.Fatal(status, b)
	}
	var n int
	d.QueryRow(`SELECT COUNT(*) FROM messages WHERE conversation_id=? AND role='assistant' AND llm_trace_json IS NULL`, cid).Scan(&n)
	if n != 1 {
		t.Fatal("default capture", n)
	}
}
