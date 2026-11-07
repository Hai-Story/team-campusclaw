package tutor

import (
	"campusclaw/backend/internal/auth"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClassTemplatesIntegration(t *testing.T) {
	d, cfg := integrationDB(t)
	if err := Migrate(d); err != nil {
		t.Fatal(err)
	}
	tag := fmt.Sprint(time.Now().UnixNano())
	res, err := d.Exec(`INSERT INTO classes(name) VALUES(?)`, "templates-"+tag)
	if err != nil {
		t.Fatal(err)
	}
	cls, _ := res.LastInsertId()
	users := []int64{}
	defer func() {
		d.Exec(`DELETE FROM conversations WHERE class_id=?`, cls)
		d.Exec(`DELETE FROM tutor_agents WHERE class_id=?`, cls)
		d.Exec(`DELETE FROM tutor_skills WHERE class_id=?`, cls)
		d.Exec(`DELETE FROM tutor_app_templates WHERE class_id=?`, cls)
		for _, id := range users {
			d.Exec(`DELETE FROM sessions WHERE user_id=?`, id)
			d.Exec(`DELETE FROM users WHERE id=?`, id)
		}
		d.Exec(`DELETE FROM classes WHERE id=?`, cls)
	}()
	tx, _ := d.Begin()
	if err = seedApps(tx, cls); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	s := NewService(d, nil, nil)
	a := auth.NewService(d, cfg)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/login", a.Login)
	mux.HandleFunc("GET /api/conversations/{id}", a.Middleware(auth.RequireStudent(s.Conversation)))
	mux.HandleFunc("GET /api/ai-apps", a.Middleware(s.Apps))
	mux.HandleFunc("PUT /api/ai-apps/{key}/template", a.Middleware(auth.RequireTeacher(s.SaveAppTemplate)))
	mux.HandleFunc("POST /api/ai-apps/{key}/launch", a.Middleware(auth.RequireStudent(s.LaunchApp)))
	server := httptest.NewServer(mux)
	defer server.Close()
	tokens := map[string]string{}
	for _, role := range []string{"teacher", "student"} {
		name := "tpl-" + role + tag
		source, pass := "student_a1", cfg.SeedStudentPassword
		if role == "teacher" {
			source, pass = "teacher_a", cfg.SeedTeacherPassword
		}
		res, err = d.Exec(`INSERT INTO users(username,password_hash,role,class_id) SELECT ?,password_hash,?,? FROM users WHERE username=?`, name, role, cls, source)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := res.LastInsertId()
		users = append(users, id)
		r, _ := http.Post(server.URL+"/api/login", "application/json", strings.NewReader(fmt.Sprintf(`{"username":%q,"password":%q}`, name, pass)))
		var login map[string]any
		json.NewDecoder(r.Body).Decode(&login)
		r.Body.Close()
		if r.StatusCode != 200 {
			t.Fatal("login")
		}
		tokens[role] = login["token"].(string)
	}
	call := func(who, method, path, data string, want int) string {
		req, _ := http.NewRequest(method, server.URL+path, strings.NewReader(data))
		req.Header.Set("Authorization", "Bearer "+tokens[who])
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		if r.StatusCode != want {
			t.Fatal(r.StatusCode, want, string(b))
		}
		return string(b)
	}
	p := "/api/ai-apps/concept/template"
	input := `{"user_template":"教师新模板：{{topic}}；我的想法：{{attempt}}","expected_version":0,"class_id":2}`
	call("student", "PUT", p, input, 403)
	call("teacher", "PUT", p, `{"user_template":"{{topic}}","expected_version":0}`, 400)
	call("teacher", "PUT", p, input, 200)
	call("teacher", "PUT", p, input, 409)
	var apps []AIApp
	json.Unmarshal([]byte(call("student", "GET", "/api/ai-apps", "", 200)), &apps)
	if apps[0].TemplateVersion != 1 || !strings.Contains(apps[0].UserTemplate, "教师新模板") {
		t.Fatal(apps[0])
	}
	if err := Migrate(d); err != nil {
		t.Fatal(err)
	}
	b := call("student", "POST", "/api/ai-apps/concept/launch", `{"fields":{"topic":"通分","attempt":"同乘"}}`, 201)
	if !strings.Contains(b, "教师新模板：通分；我的想法：同乘") {
		t.Fatal(b)
	}
	var n int
	d.QueryRow(`SELECT COUNT(*) FROM tutor_app_templates WHERE class_id=2 AND user_template LIKE '教师新模板%'`).Scan(&n)
	if n != 0 {
		t.Fatal("class overridden")
	}
	// An old app answer must not expose unrelated stored reference cards; raw audit data stays unchanged.
	var uid int64
	d.QueryRow(`SELECT id FROM users WHERE username=?`, "tpl-student"+tag).Scan(&uid)
	result, err := d.Exec(`INSERT INTO conversations(class_id,user_id,agent_id) VALUES(?,?,?)`, cls, uid, apps[1].AgentID)
	if err != nil {
		t.Fatal(err)
	}
	cid, _ := result.LastInsertId()
	d.Exec(`INSERT INTO messages(conversation_id,role,content,citations_json) VALUES(?,'user','递伞动作和雨幕细节有什么作用？','[]')`, cid)
	result, err = d.Exec(`INSERT INTO messages(conversation_id,role,content,citations_json) VALUES(?,'assistant','旧回答错误引用[1]。','[{"index":1,"material_id":1,"material_title":"科学课程资料","excerpt":"磁铁能吸铁钉"}]')`, cid)
	if err != nil {
		t.Fatal(err)
	}
	mid, _ := result.LastInsertId()
	visible := call("student", "GET", fmt.Sprintf("/api/conversations/%d", cid), "", 200)
	if strings.Contains(visible, "科学课程资料") || strings.Contains(visible, "[1]") {
		t.Fatal(visible)
	}
	var stored string
	d.QueryRow(`SELECT citations_json FROM messages WHERE id=?`, mid).Scan(&stored)
	if !strings.Contains(stored, "科学课程资料") {
		t.Fatal("audit rewritten")
	}

}
