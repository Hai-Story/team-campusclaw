package tutor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"campusclaw/backend/internal/auth"
	"campusclaw/backend/internal/retrieval"
)

func TestDeletionIntegration(t *testing.T) {
	d, cfg := integrationDB(t)
	if e := Migrate(d); e != nil {
		t.Fatal(e)
	}
	gate := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(gate)
		}
	}()
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"保留的回答\"}}]}\n\n")
		w.(http.Flusher).Flush()
		select {
		case <-gate:
		case <-r.Context().Done():
			return
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer model.Close()
	s := NewService(d, &retrieval.Searcher{DB: d, Cfg: cfg}, &retrieval.ChatClient{BaseURL: model.URL, Model: "fixture"})
	a := auth.NewService(d, cfg)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/login", a.Login)
	mux.HandleFunc("POST /api/logout", a.Logout)
	mux.HandleFunc("DELETE /api/agents/{id}", a.Middleware(auth.RequireTeacher(s.DeleteAgent)))
	mux.HandleFunc("PUT /api/agents/{id}", a.Middleware(auth.RequireTeacher(s.SaveAgent)))
	mux.HandleFunc("POST /api/agents", a.Middleware(auth.RequireTeacher(s.SaveAgent)))
	mux.HandleFunc("GET /api/agents/{id}", a.Middleware(s.Agent))
	mux.HandleFunc("GET /api/agents", a.Middleware(s.Agents))
	mux.HandleFunc("POST /api/conversations", a.Middleware(auth.RequireStudent(s.CreateConversation)))
	mux.HandleFunc("GET /api/conversations/{id}", a.Middleware(auth.RequireStudent(s.Conversation)))
	mux.HandleFunc("POST /api/conversations/{id}/messages", a.Middleware(auth.RequireStudent(s.SendMessage)))
	mux.HandleFunc("DELETE /api/conversations/{id}", a.Middleware(auth.RequireStudent(s.DeleteConversation)))
	server := httptest.NewServer(mux)
	defer server.Close()
	tag := fmt.Sprint(time.Now().UnixNano())
	users := []int64{}
	convs := []int64{}
	agents := []int64{}
	tokens := map[string]string{}
	t.Cleanup(func() {
		for _, id := range convs {
			d.Exec(`DELETE FROM mcp_tool_calls WHERE conversation_id=?`, id)
			d.Exec(`DELETE FROM conversations WHERE id=?`, id)
		}
		for _, id := range agents {
			d.Exec(`DELETE FROM tutor_agents WHERE id=?`, id)
		}
		for _, id := range users {
			d.Exec(`DELETE FROM sessions WHERE user_id=?`, id)
			d.Exec(`DELETE FROM users WHERE id=?`, id)
		}
	})
	for _, f := range []struct {
		key, source, role string
		class             int
	}{{"peer", "student_a1", "student", 1}, {"teacherB", "teacher_a", "teacher", 2}} {
		name := "delete_" + f.key + tag
		r, e := d.Exec(`INSERT INTO users(username,password_hash,role,class_id) SELECT ?,password_hash,?,? FROM users WHERE username=?`, name, f.role, f.class, f.source)
		if e != nil {
			t.Fatal(e)
		}
		id, _ := r.LastInsertId()
		users = append(users, id)
	}
	login := func(who, name, pass string) {
		b, _ := json.Marshal(map[string]string{"username": name, "password": pass})
		resp, e := http.Post(server.URL+"/api/login", "application/json", bytes.NewReader(b))
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		var v struct {
			Token string `json:"token"`
		}
		json.NewDecoder(resp.Body).Decode(&v)
		if resp.StatusCode != 200 || v.Token == "" {
			t.Fatal("fixture login failed", resp.StatusCode)
		}
		tokens[who] = v.Token
	}
	login("teacher", "teacher_a", cfg.SeedTeacherPassword)
	login("student", "student_a1", cfg.SeedStudentPassword)
	login("other", "student_b1", cfg.SeedStudentPassword)
	login("peer", "delete_peer"+tag, cfg.SeedStudentPassword)
	login("teacherB", "delete_teacherB"+tag, cfg.SeedTeacherPassword)
	defer func() {
		for _, token := range tokens {
			req, _ := http.NewRequest("POST", server.URL+"/api/logout", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			res, e := http.DefaultClient.Do(req)
			if e == nil {
				res.Body.Close()
			}
		}
	}()
	call := func(method, path, who string, payload any, want int) []byte {
		t.Helper()
		var b io.Reader
		if payload != nil {
			data, _ := json.Marshal(payload)
			b = bytes.NewReader(data)
		}
		req, _ := http.NewRequest(method, server.URL+path, b)
		req.Header.Set("Content-Type", "application/json")
		if token := tokens[who]; token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		out, _ := io.ReadAll(res.Body)
		if res.StatusCode != want {
			t.Fatalf("%s %s want%d got%d", method, path, want, res.StatusCode)
		}
		return out
	}
	name := "删除验收" + tag
	makeAgent := func() int64 {
		var v Agent
		json.Unmarshal(call("POST", "/api/agents", "teacher", AgentInput{Name: name, Enabled: true}, 201), &v)
		agents = append(agents, v.ID)
		return v.ID
	}
	aid := makeAgent()
	ap := fmt.Sprintf("/api/agents/%d", aid)
	call("DELETE", ap, "", nil, 401)
	call("DELETE", ap, "student", nil, 403)
	cross := call("DELETE", ap, "teacherB", nil, 404)
	absent := call("DELETE", "/api/agents/999999999", "teacherB", nil, 404)
	if string(cross) != string(absent) {
		t.Fatal("cross-class deletion reveals existence")
	}
	var uid int64
	d.QueryRow(`SELECT id FROM users WHERE username='student_a1'`).Scan(&uid)
	for i := 0; i < 2; i++ {
		var c Conversation
		json.Unmarshal(call("POST", "/api/conversations", "student", map[string]int64{"agent_id": aid}, 201), &c)
		convs = append(convs, c.ID)
	}
	cp := fmt.Sprintf("/api/conversations/%d", convs[0])
	call("DELETE", cp, "", nil, 401)
	call("DELETE", cp, "teacher", nil, 403)
	for _, who := range []string{"peer", "other"} {
		if string(call("DELETE", cp, who, nil, 404)) != string(call("DELETE", "/api/conversations/999999999", who, nil, 404)) {
			t.Fatal("private conversation leaked")
		}
	}
	// Both streams share an agent read gate, but have independent conversation locks.
	streams := []*http.Response{}
	for _, cid := range convs {
		req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/conversations/%d/messages", server.URL, cid), strings.NewReader(`{"content":"检查一下思路"}`))
		req.Header.Set("Authorization", "Bearer "+tokens["student"])
		req.Header.Set("Content-Type", "application/json")
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		if res.StatusCode != 200 {
			t.Fatal("parallel generation failed", res.StatusCode)
		}
		streams = append(streams, res)
	}
	call("DELETE", ap, "teacher", nil, 409)
	call("DELETE", cp, "student", nil, 409)
	close(gate)
	released = true
	for _, res := range streams {
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
	}
	// Attach a skill binding and an audit row to verify exact deletion boundaries.
	r, e := d.Exec(`INSERT INTO tutor_skills(class_id,name,trigger_text,description,tools_json,source_text,enabled) VALUES(1,?,'测试','说明','[]','说明',TRUE)`, "删除技能"+tag)
	if e != nil {
		t.Fatal(e)
	}
	sid, _ := r.LastInsertId()
	t.Cleanup(func() { d.Exec(`DELETE FROM tutor_skills WHERE id=?`, sid) })
	if _, e = d.Exec(`INSERT INTO tutor_agent_skills VALUES(?,?)`, aid, sid); e != nil {
		t.Fatal(e)
	}
	if _, e = d.Exec(`INSERT INTO tutor_agent_materials SELECT ?,id FROM materials WHERE class_id=1 LIMIT 1`, aid); e != nil {
		t.Fatal(e)
	}
	if _, e = d.Exec(`INSERT INTO mcp_tool_calls(class_id,server_id,server_name,tool_name,tool_ref,user_id,conversation_id,request_message_id,round_number,status,summary) VALUES(1,0,'fixture','hint','hint@fixture',?,?,0,1,'ok','保留审计')`, uid, convs[0]); e != nil {
		t.Fatal(e)
	}
	call("DELETE", ap, "teacher", nil, 204)
	call("GET", ap, "teacher", nil, 404)
	call("GET", ap, "student", nil, 404)
	call("DELETE", ap, "teacher", nil, 404)
	call("PUT", ap, "teacher", AgentInput{Name: name, Enabled: true}, 404)
	for _, table := range []string{"tutor_agent_skills", "tutor_agent_materials"} {
		var n int
		d.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE agent_id=?`, aid).Scan(&n)
		if n != 0 {
			t.Fatal("binding retained", table)
		}
	}
	for _, who := range []string{"teacher", "student"} {
		var list []Agent
		json.Unmarshal(call("GET", "/api/agents", who, nil, 200), &list)
		for _, v := range list {
			if v.ID == aid {
				t.Fatal("deleted agent visible")
			}
		}
	}
	replacement := makeAgent()
	if replacement == aid {
		t.Fatal("history agent replaced")
	}
	call("POST", "/api/conversations", "student", map[string]int64{"agent_id": aid}, 404)
	var detail struct {
		Conversation Conversation `json:"conversation"`
		Messages     []Message    `json:"messages"`
	}
	json.Unmarshal(call("GET", cp, "student", nil, 200), &detail)
	if !detail.Conversation.AgentDeleted || detail.Conversation.AgentName != name || len(detail.Messages) != 2 {
		t.Fatal("historical conversation lost")
	}
	call("POST", cp+"/messages", "student", map[string]string{"content": "不应继续生成"}, 409)
	var count int
	d.QueryRow(`SELECT COUNT(*) FROM messages WHERE conversation_id=?`, convs[0]).Scan(&count)
	if count != 2 {
		t.Fatal("deleted-agent send mutated messages")
	}
	call("DELETE", cp, "student", nil, 204)
	call("GET", cp, "student", nil, 404)
	call("DELETE", cp, "student", nil, 404)
	call("POST", cp+"/messages", "student", map[string]string{"content": "不应复活"}, 404)
	d.QueryRow(`SELECT COUNT(*) FROM messages WHERE conversation_id=?`, convs[0]).Scan(&count)
	if count != 0 {
		t.Fatal("messages not cascaded")
	}
	d.QueryRow(`SELECT COUNT(*) FROM mcp_tool_calls WHERE conversation_id=?`, convs[0]).Scan(&count)
	if count != 1 {
		t.Fatal("audit removed")
	}
	call("GET", fmt.Sprintf("/api/conversations/%d", convs[1]), "student", nil, 200)
}

func TestDeletedSeedAndMigrationIntegration(t *testing.T) {
	d, _ := integrationDB(t)
	if e := Migrate(d); e != nil {
		t.Fatal(e)
	}
	r, e := d.Exec(`INSERT INTO classes(name) VALUES(?)`, fmt.Sprint("删除种子验收", time.Now().UnixNano()))
	if e != nil {
		t.Fatal(e)
	}
	class, _ := r.LastInsertId()
	t.Cleanup(func() {
		d.Exec(`DELETE FROM tutor_agents WHERE class_id=?`, class)
		d.Exec(`DELETE FROM tutor_skills WHERE class_id=?`, class)
		d.Exec(`DELETE FROM classes WHERE id=?`, class)
	})
	seed := func() {
		tx, e := d.BeginTx(context.Background(), nil)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback()
		if e = seedClass(tx, class); e != nil {
			t.Fatal(e)
		}
		if e = tx.Commit(); e != nil {
			t.Fatal(e)
		}
	}
	seed()
	var id int64
	d.QueryRow(`SELECT id FROM tutor_agents WHERE class_id=? AND seed_key='guidance'`, class).Scan(&id)
	if _, e = d.Exec(`UPDATE tutor_agents SET deleted_at=CURRENT_TIMESTAMP,enabled=FALSE WHERE id=?`, id); e != nil {
		t.Fatal(e)
	}
	seed()
	if e = Migrate(d); e != nil {
		t.Fatal(e)
	}
	seed()
	var n int
	d.QueryRow(`SELECT COUNT(*) FROM tutor_agents WHERE class_id=? AND seed_key IN ('guidance','explain')`, class).Scan(&n)
	if n != 2 {
		t.Fatal("seed resurrected")
	}
	var deleted bool
	d.QueryRow(`SELECT deleted_at IS NOT NULL FROM tutor_agents WHERE id=?`, id).Scan(&deleted)
	if !deleted {
		t.Fatal("deletion lost")
	}
	if _, e = d.Exec(`INSERT INTO tutor_agents(class_id,name,prompt) VALUES(?,'解题助手','新定义')`, class); e != nil {
		t.Fatal("same-name replacement failed", e)
	}
}
