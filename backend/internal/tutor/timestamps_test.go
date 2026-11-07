package tutor

import (
	"campusclaw/backend/internal/auth"
	"campusclaw/backend/internal/retrieval"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestTimestampsIntegration(t *testing.T) {
	d, cfg := integrationDB(t)
	if err := Migrate(d); err != nil {
		t.Fatal(err)
	}
	var partial atomic.Bool
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []map[string]any `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		for _, m := range payload.Messages {
			if _, ok := m["created_at"]; ok {
				t.Error("time entered LLM request")
			}
			if strings.Contains(fmt.Sprint(m["content"]), "2020-01-01") {
				t.Error("history timestamp entered content")
			}
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"时间验证回答\"}}]}\n\n")
		if !partial.Load() {
			fmt.Fprint(w, "data: [DONE]\n\n")
		}
	}))
	defer model.Close()
	s := NewService(d, &retrieval.Searcher{DB: d, Cfg: cfg}, &retrieval.ChatClient{BaseURL: model.URL, APIKey: "fixture", Model: "fixture"})
	a := auth.NewService(d, cfg)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/login", a.Login)
	mux.HandleFunc("GET /api/conversations", a.Middleware(auth.RequireStudent(s.Conversations)))
	mux.HandleFunc("POST /api/conversations/{id}/messages", a.Middleware(auth.RequireStudent(s.SendMessage)))
	server := httptest.NewServer(mux)
	defer server.Close()
	insert := func(query string, args ...any) int64 {
		t.Helper()
		r, e := d.Exec(query, args...)
		if e != nil {
			t.Fatal(e)
		}
		id, e := r.LastInsertId()
		if e != nil {
			t.Fatal(e)
		}
		return id
	}
	name := "time-fixture" + fmt.Sprint(time.Now().UnixNano())
	uid := insert(`INSERT INTO users(username,password_hash,role,class_id) SELECT ?,password_hash,'student',1 FROM users WHERE username='student_a1'`, name)
	defer d.Exec(`DELETE FROM users WHERE id=?`, uid)
	aid := insert(`INSERT INTO tutor_agents(class_id,name,prompt) VALUES(1,?,'时间验证助手')`, name)
	defer d.Exec(`DELETE FROM tutor_agents WHERE id=?`, aid)
	cid := insert(`INSERT INTO conversations(class_id,user_id,agent_id,created_at,updated_at) VALUES(1,?,?,FROM_UNIXTIME(946684800),FROM_UNIXTIME(946684800))`, uid, aid)
	defer d.Exec(`DELETE FROM conversations WHERE id=?`, cid)
	cid2 := insert(`INSERT INTO conversations(class_id,user_id,agent_id,created_at,updated_at) VALUES(1,?,?,FROM_UNIXTIME(1262304000),FROM_UNIXTIME(1262304000))`, uid, aid)
	defer d.Exec(`DELETE FROM conversations WHERE id=?`, cid2)
	insert(`INSERT INTO messages(conversation_id,role,content,citations_json,created_at) VALUES(?,'assistant','历史回答','[]',FROM_UNIXTIME(1577836800))`, cid)
	u := auth.User{ID: uid, ClassID: 1}
	c, err := s.conversation(context.Background(), cid, u)
	if err != nil {
		t.Fatal(err)
	}
	if c.Created != "2000-01-01T00:00:00Z" || c.Updated != "2020-01-01T00:00:00Z" {
		t.Fatal(c)
	}
	// The same stored TIMESTAMP has the same absolute JSON time in another SQL session timezone.
	conn, err := d.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var previousZone string
	conn.QueryRowContext(context.Background(), `SELECT @@session.time_zone`).Scan(&previousZone)
	if _, err = conn.ExecContext(context.Background(), `SET time_zone='+08:00'`); err != nil {
		t.Fatal(err)
	}
	c2, err := scanConversation(conn.QueryRowContext(context.Background(), `SELECT `+conversationTimeColumns+` FROM conversations c JOIN tutor_agents a ON a.id=c.agent_id WHERE c.id=?`, cid))
	conn.ExecContext(context.Background(), `SET time_zone=?`, previousZone)
	conn.Close()
	if err != nil || c2.Created != c.Created || c2.Updated != c.Updated {
		t.Fatal(c2, err)
	}
	res, err := http.Post(server.URL+"/api/login", "application/json", strings.NewReader(fmt.Sprintf(`{"username":%q,"password":%q}`, name, cfg.SeedStudentPassword)))
	if err != nil {
		t.Fatal(err)
	}
	var login map[string]string
	json.NewDecoder(res.Body).Decode(&login)
	res.Body.Close()
	token := login["token"]
	if token == "" {
		t.Fatal("login")
	}
	defer d.Exec(`DELETE FROM sessions WHERE token_hash=SHA2(?,256)`, token)
	call := func(method, path, body string) string {
		t.Helper()
		req, _ := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		if res.StatusCode != 200 {
			t.Fatal(res.StatusCode, string(b))
		}
		return string(b)
	}
	var list []Conversation
	if err = json.Unmarshal([]byte(call("GET", "/api/conversations", "")), &list); err != nil || len(list) != 2 || list[0].ID != cid {
		t.Fatal(list, err)
	}
	for _, broken := range []bool{false, true} {
		partial.Store(broken)
		stream := call("POST", fmt.Sprintf("/api/conversations/%d/messages", cid), `{"content":"请检查这个思路","created_at":"2099-01-01T00:00:00Z"}`)
		var meta struct {
			User      string `json:"user_created_at"`
			Assistant string `json:"assistant_created_at"`
		}
		for _, line := range strings.Split(stream, "\n") {
			if strings.HasPrefix(line, "data:") && strings.Contains(line, "assistant_created_at") {
				if err = json.Unmarshal([]byte(strings.TrimPrefix(line, "data:")), &meta); err != nil {
					t.Fatal(err)
				}
			}
		}
		history, err := s.messages(context.Background(), cid, false)
		if err != nil {
			t.Fatal(err)
		}
		user, reply := history[len(history)-2], history[len(history)-1]
		if meta.User == "" || meta.Assistant == "" || user.Created != meta.User || reply.Created != meta.Assistant || strings.HasPrefix(user.Created, "2099") {
			t.Fatal(meta, user, reply)
		}
		if _, err = time.Parse(time.RFC3339, reply.Created); err != nil {
			t.Fatal(err)
		}
		if (reply.Status == "partial") != broken {
			t.Fatal(reply.Status, stream)
		}
		if !strings.Contains(stream, `"created_at":"`+meta.Assistant+`"`) {
			t.Fatal("terminal time absent")
		}
	}
}
