package tutor

import (
	"campusclaw/backend/internal/auth"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAppTemplates(t *testing.T) {
	for _, app := range appDefinitions() {
		fields := map[string]string{}
		for _, f := range app.Fields {
			fields[f.Key] = "我的输入"
		}
		q, err := renderApp(app, fields)
		if err != nil || strings.Contains(q, "{{") || len([]rune(q)) > 500 {
			t.Fatal(q, err)
		}
		if _, err = renderApp(app, map[string]string{}); err == nil {
			t.Fatal("missing accepted")
		}
		fields[app.Fields[0].Key] = strings.Repeat("字", 181)
		if _, err = renderApp(app, fields); err == nil {
			t.Fatal("oversize accepted")
		}
		source, _ := builtinSkills.ReadFile("builtins/" + app.Key + "/SKILL.md")
		if _, err = ParseSkill(string(source)); err != nil {
			t.Fatal(err)
		}
	}
}
func TestTemplateInputIsNotReinterpreted(t *testing.T) {
	app := appDefinitions()[0]
	q, err := renderApp(app, map[string]string{"topic": "保留{{attempt}}原文", "attempt": "另一字段"})
	if err != nil || !strings.Contains(q, "保留{{attempt}}原文") {
		t.Fatal(q, err)
	}
}
func TestAppsIntegration(t *testing.T) {
	d, cfg := integrationDB(t)
	if err := Migrate(d); err != nil {
		t.Fatal(err)
	}
	if err := Seed(d); err != nil {
		t.Fatal(err)
	}
	svc := NewService(d, nil, nil)
	authSvc := auth.NewService(d, cfg)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/login", authSvc.Login)
	mux.HandleFunc("GET /api/ai-apps", authSvc.Middleware(svc.Apps))
	mux.HandleFunc("POST /api/ai-apps/{key}/launch", authSvc.Middleware(auth.RequireStudent(svc.LaunchApp)))
	server := httptest.NewServer(mux)
	defer server.Close()
	tokens := map[string]string{}
	for _, u := range []string{"student_a1", "student_b1", "teacher_a"} {
		pass := cfg.SeedStudentPassword
		if u == "teacher_a" {
			pass = cfg.SeedTeacherPassword
		}
		res, _ := http.Post(server.URL+"/api/login", "application/json", strings.NewReader(fmt.Sprintf(`{"username":%q,"password":%q}`, u, pass)))
		var v map[string]any
		json.NewDecoder(res.Body).Decode(&v)
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatal("login", res.StatusCode)
		}
		tokens[u] = v["token"].(string)
		defer d.Exec(`DELETE FROM sessions WHERE token_hash=SHA2(?,256)`, tokens[u])
	}
	request := func(who, method, path, body string) (int, []byte) {
		req, _ := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tokens[who])
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var b json.RawMessage
		json.NewDecoder(res.Body).Decode(&b)
		return res.StatusCode, b
	}
	status, b := request("student_a1", "GET", "/api/ai-apps", "")
	var a []AIApp
	json.Unmarshal(b, &a)
	if status != 200 || len(a) != 4 || a[0].AgentID == 0 {
		t.Fatal(status, string(b))
	}
	_, b = request("student_b1", "GET", "/api/ai-apps", "")
	var other []AIApp
	json.Unmarshal(b, &other)
	if other[0].AgentID == a[0].AgentID {
		t.Fatal("cross-class catalog")
	}
	status, b = request("student_a1", "POST", "/api/ai-apps/concept/launch", `{"fields":{"topic":"通分的概念","attempt":"我不理解"}}`)
	if status != 201 {
		t.Fatal(status, string(b))
	}
	var launch struct {
		Conversation Conversation `json:"conversation"`
		Prompt       string       `json:"prompt"`
	}
	json.Unmarshal(b, &launch)
	defer d.Exec(`DELETE FROM conversations WHERE id=?`, launch.Conversation.ID)
	if launch.Conversation.AgentID != a[0].AgentID || !strings.Contains(launch.Prompt, "通分") {
		t.Fatal(string(b))
	}
	status, _ = request("teacher_a", "POST", "/api/ai-apps/concept/launch", `{"fields":{}}`)
	if status != 403 {
		t.Fatal(status)
	}
	status, _ = request("student_a1", "POST", "/api/ai-apps/concept/launch", `{"fields":{}}`)
	if status != 400 {
		t.Fatal(status)
	}
	// Use an isolated class in a transaction to verify restart idempotence without modifying demo configuration.
	tx, _ := d.Begin()
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO classes(name) VALUES(?)`, fmt.Sprintf("app-test-%d", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	classID, _ := res.LastInsertId()
	if err = seedApps(tx, classID); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(`UPDATE tutor_agents SET prompt='teacher-edit',enabled=FALSE,deleted_at=CURRENT_TIMESTAMP WHERE class_id=? AND seed_key='app-concept'`, classID)
	if err != nil {
		t.Fatal(err)
	}
	if err = seedApps(tx, classID); err != nil {
		t.Fatal(err)
	}
	var prompt string
	var deleted, enabled bool
	if err = tx.QueryRow(`SELECT prompt,deleted_at IS NOT NULL,enabled FROM tutor_agents WHERE class_id=? AND seed_key='app-concept'`, classID).Scan(&prompt, &deleted, &enabled); err != nil || prompt != "teacher-edit" || !deleted || enabled {
		t.Fatal(prompt, deleted, enabled, err)
	}
}

func TestAppLaunchAvailabilityIntegration(t *testing.T) {
	d, cfg := integrationDB(t)
	if err := Migrate(d); err != nil {
		t.Fatal(err)
	}
	tag := fmt.Sprint(time.Now().UnixNano())
	res, err := d.Exec(`INSERT INTO classes(name) VALUES(?)`, "launch-test-"+tag)
	if err != nil {
		t.Fatal(err)
	}
	classID, _ := res.LastInsertId()
	defer d.Exec(`DELETE FROM classes WHERE id=?`, classID)
	defer d.Exec(`DELETE FROM tutor_skills WHERE class_id=?`, classID)
	defer d.Exec(`DELETE FROM tutor_agents WHERE class_id=?`, classID)
	tx, _ := d.Begin()
	if err = seedApps(tx, classID); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	name := "app-student-" + tag
	res, err = d.Exec(`INSERT INTO users(username,password_hash,role,class_id) SELECT ?,password_hash,'student',? FROM users WHERE username='student_a1'`, name, classID)
	if err != nil {
		t.Fatal(err)
	}
	uid, _ := res.LastInsertId()
	defer d.Exec(`DELETE FROM users WHERE id=?`, uid)
	defer d.Exec(`DELETE FROM sessions WHERE user_id=?`, uid)
	s := NewService(d, nil, nil)
	a := auth.NewService(d, cfg)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/login", a.Login)
	mux.HandleFunc("POST /api/ai-apps/{key}/launch", a.Middleware(auth.RequireStudent(s.LaunchApp)))
	server := httptest.NewServer(mux)
	defer server.Close()
	r, _ := http.Post(server.URL+"/api/login", "application/json", strings.NewReader(fmt.Sprintf(`{"username":%q,"password":%q}`, name, cfg.SeedStudentPassword)))
	var login map[string]any
	json.NewDecoder(r.Body).Decode(&login)
	r.Body.Close()
	if r.StatusCode != 200 {
		t.Fatal("login")
	}
	request := func(want int) {
		req, _ := http.NewRequest("POST", server.URL+"/api/ai-apps/concept/launch", strings.NewReader(`{"fields":{"topic":"概念","attempt":"尝试"}}`))
		req.Header.Set("Authorization", "Bearer "+login["token"].(string))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != want {
			t.Fatal(res.StatusCode, want)
		}
	}
	d.Exec(`UPDATE tutor_agents SET enabled=FALSE WHERE class_id=? AND seed_key='app-concept'`, classID)
	request(400)
	d.Exec(`UPDATE tutor_agents SET deleted_at=CURRENT_TIMESTAMP WHERE class_id=? AND seed_key='app-concept'`, classID)
	request(404)
	var n int
	d.QueryRow(`SELECT COUNT(*) FROM conversations WHERE class_id=?`, classID).Scan(&n)
	if n != 0 {
		t.Fatal("unavailable launch created conversation")
	}
	// Simulate the exact v1 default, then verify both its upgrade and preservation of teacher changes.
	source, _ := builtinSkills.ReadFile("builtins/reading/SKILL.md")
	sk, _ := ParseSkill(string(source))
	old := strings.Replace(sk.Source, "subject: 语文\n", "", 1)
	d.Exec(`UPDATE tutor_skills SET source_text=?,subject_text='' WHERE class_id=? AND seed_key='app-reading'`, old, classID)
	tx, _ = d.Begin()
	if err = seedApps(tx, classID); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	tx.Commit()
	var subject string
	d.QueryRow(`SELECT subject_text FROM tutor_skills WHERE class_id=? AND seed_key='app-reading'`, classID).Scan(&subject)
	if subject != "语文" {
		t.Fatal("default not upgraded")
	}
	edited := old + "教师修订"
	d.Exec(`UPDATE tutor_skills SET source_text=?,subject_text='' WHERE class_id=? AND seed_key='app-reading'`, edited, classID)
	tx, _ = d.Begin()
	if err = seedApps(tx, classID); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	tx.Commit()
	var saved string
	d.QueryRow(`SELECT source_text FROM tutor_skills WHERE class_id=? AND seed_key='app-reading'`, classID).Scan(&saved)
	if saved != edited {
		t.Fatal("teacher edit overwritten")
	}
}
