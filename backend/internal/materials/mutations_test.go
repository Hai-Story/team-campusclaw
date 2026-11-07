package materials

import (
	"bytes"
	"campusclaw/backend/internal/auth"
	"campusclaw/backend/internal/chunking"
	"campusclaw/backend/internal/config"
	"campusclaw/backend/internal/db"
	"campusclaw/backend/internal/retrieval"
	"campusclaw/backend/internal/tutor"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMaterialLifecycleIntegration(t *testing.T) {
	if os.Getenv("TUTOR_INTEGRATION") != "1" {
		t.Skip("requires compose DB")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	d, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err = db.Migrate(d); err != nil {
		t.Fatal(err)
	}
	if err = tutor.Migrate(d); err != nil {
		t.Fatal(err)
	}
	cfg.UploadDir = t.TempDir()
	cfg.MaxUploadBytes = 1024
	var failEmbed atomic.Bool
	var mu sync.Mutex
	inputs := []string{}
	points := map[int64]int64{}
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var v map[string]any
		if r.Body != nil {
			json.NewDecoder(r.Body).Decode(&v)
		}
		if strings.HasSuffix(r.URL.Path, "/embeddings") {
			if failEmbed.Load() {
				w.WriteHeader(500)
				fmt.Fprint(w, `{"error":"fixture failure"}`)
				return
			}
			mu.Lock()
			inputs = append(inputs, fmt.Sprint(v["input"]))
			mu.Unlock()
			fmt.Fprint(w, `{"data":[{"embedding":[1]}]}`)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if strings.HasSuffix(r.URL.Path, "/points") && r.Method == "PUT" {
			for _, p := range v["points"].([]any) {
				point := p.(map[string]any)
				points[int64(point["id"].(float64))] = int64(point["payload"].(map[string]any)["material_id"].(float64))
			}
		}
		if strings.HasSuffix(r.URL.Path, "/points/delete") {
			filter := v["filter"].(map[string]any)["must"].([]any)[0].(map[string]any)
			id := int64(filter["match"].(map[string]any)["value"].(float64))
			for pid, mid := range points {
				if mid == id {
					delete(points, pid)
				}
			}
		}
		fmt.Fprint(w, `{"result":{}}`)
	}))
	defer remote.Close()
	ix := &retrieval.Indexer{DB: d, Cfg: cfg, Embed: &retrieval.EmbeddingClient{BaseURL: remote.URL, Dim: 1}, Qdr: &retrieval.QdrantClient{BaseURL: remote.URL, Dim: 1}}
	s := NewService(d, cfg)
	s.SetIndexer(ix)
	a := auth.NewService(d, cfg)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/login", a.Login)
	mux.HandleFunc("POST /api/materials", a.Middleware(auth.RequireTeacher(s.Upload)))
	mux.HandleFunc("GET /api/materials/{id}", a.Middleware(s.Detail))
	mux.HandleFunc("GET /api/materials/{id}/file", a.Middleware(s.File))
	mux.HandleFunc("PUT /api/materials/{id}", a.Middleware(auth.RequireTeacher(s.Update)))
	mux.HandleFunc("DELETE /api/materials/{id}", a.Middleware(auth.RequireTeacher(s.Delete)))
	server := httptest.NewServer(mux)
	defer server.Close()
	tag := fmt.Sprint(time.Now().UnixNano())
	res, err := d.Exec(`INSERT INTO users(username,password_hash,role,class_id) SELECT ?,password_hash,'teacher',2 FROM users WHERE username='teacher_a'`, "mat-teacherB-"+tag)
	if err != nil {
		t.Fatal(err)
	}
	otherID, _ := res.LastInsertId()
	tokens := map[string]string{}
	var mid, aid int64
	names := []string{}
	defer func() {
		d.Exec(`DELETE FROM knowledge_chunks WHERE material_id=?`, mid)
		d.Exec(`DELETE FROM knowledge_entries WHERE material_id=?`, mid)
		d.Exec(`DELETE FROM tutor_agents WHERE id=?`, aid)
		d.Exec(`DELETE FROM materials WHERE id=?`, mid)
		for _, name := range names {
			d.Exec(`DELETE FROM material_seed_suppressions WHERE class_id=1 AND title=?`, name)
		}
		for _, token := range tokens {
			d.Exec(`DELETE FROM sessions WHERE token_hash=SHA2(?,256)`, token)
		}
		d.Exec(`DELETE FROM users WHERE id=?`, otherID)
	}()
	for key, name := range map[string]string{"teacher": "teacher_a", "student": "student_a1", "other": "mat-teacherB-" + tag} {
		pass := cfg.SeedTeacherPassword
		if key == "student" {
			pass = cfg.SeedStudentPassword
		}
		r, _ := http.Post(server.URL+"/api/login", "application/json", strings.NewReader(fmt.Sprintf(`{"username":%q,"password":%q}`, name, pass)))
		var out map[string]any
		json.NewDecoder(r.Body).Decode(&out)
		r.Body.Close()
		if r.StatusCode != 200 {
			t.Fatal("login failed")
		}
		tokens[key] = out["token"].(string)
	}
	call := func(who, method, path string, data any, want int) string {
		var body io.Reader
		if data != nil {
			b, _ := json.Marshal(data)
			body = bytes.NewReader(b)
		}
		r, _ := http.NewRequest(method, server.URL+path, body)
		r.Header.Set("Authorization", "Bearer "+tokens[who])
		r.Header.Set("Content-Type", "application/json")
		out, e := http.DefaultClient.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		defer out.Body.Close()
		b, _ := io.ReadAll(out.Body)
		if out.StatusCode != want {
			t.Fatalf("%s %s: %d want%d %s", method, path, out.StatusCode, want, string(b))
		}
		return string(b)
	}
	var b bytes.Buffer
	form := multipart.NewWriter(&b)
	name := "数学维护旧版" + tag + ".md"
	names = append(names, name)
	f, _ := form.CreateFormFile("file", name)
	f.Write([]byte("旧词唯一标记：去括号只检查旧步骤。"))
	form.Close()
	req, _ := http.NewRequest("POST", server.URL+"/api/materials", &b)
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+tokens["teacher"])
	out, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var uploaded struct {
		ID int64 `json:"id"`
	}
	json.NewDecoder(out.Body).Decode(&uploaded)
	out.Body.Close()
	if out.StatusCode != 201 {
		t.Fatal(out.StatusCode)
	}
	mid = uploaded.ID
	p := fmt.Sprintf("/api/materials/%d", mid)
	res, err = d.Exec(`INSERT INTO tutor_agents(class_id,name,prompt) VALUES(1,?,'fixture')`, "mat-bind-"+tag)
	if err != nil {
		t.Fatal(err)
	}
	aid, _ = res.LastInsertId()
	d.Exec(`INSERT INTO tutor_agent_materials(agent_id,material_id) VALUES(?,?)`, aid, mid)
	valid := map[string]any{"title": "数学维护新版" + tag, "body": "新词唯一标记：去括号时应当使用乘法分配律。", "version": 1}
	names = append(names, valid["title"].(string))
	call("student", "PUT", p, valid, 403)
	cross := call("other", "PUT", p, valid, 404)
	missing := call("other", "PUT", "/api/materials/999999999", valid, 404)
	if cross != missing {
		t.Fatal("cross class differs")
	}
	call("teacher", "PUT", p, map[string]any{"title": "x", "body": " ", "version": 1}, 400)
	call("teacher", "PUT", p, map[string]any{"title": "x", "body": "ok", "version": 999}, 409)
	call("teacher", "PUT", p, map[string]any{"title": "x", "body": strings.Repeat("x", 1025), "version": 1}, 413)
	response := call("teacher", "PUT", p, valid, 200)
	if !strings.Contains(response, `"index_status":"ready"`) {
		t.Fatal(response)
	}
	body := call("student", "GET", p, nil, 200)
	if !strings.Contains(body, "新词唯一标记") || strings.Contains(body, "旧词唯一标记") || !strings.Contains(body, `"version":2`) {
		t.Fatal(body)
	}
	if got := call("student", "GET", p+"/file", nil, 200); got != valid["body"] {
		t.Fatal("download stale")
	}
	search := retrieval.Searcher{DB: d, Cfg: cfg}
	hits, err := search.Search(context.Background(), 1, "旧词唯一标记", "keyword", 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hits {
		if h.MaterialID == mid && strings.Contains(h.Excerpt, "旧词唯一标记") {
			t.Fatal("old searchable text")
		}
	}
	// A queued old caller must re-read the current DB source rather than replay its arguments.
	if _, err = ix.IndexMaterial(context.Background(), mid, 1, 1, "旧词唯一标记", "auto", chunking.Options{}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	last := inputs[len(inputs)-1]
	mu.Unlock()
	if !strings.Contains(last, "新词唯一标记") {
		t.Fatal("stale reindex arguments won")
	}
	failEmbed.Store(true)
	valid["version"] = 2
	valid["body"] = "已保存但嵌入失败的当前正文。"
	response = call("teacher", "PUT", p, valid, 200)
	if !strings.Contains(response, `"index_status":"failed"`) {
		t.Fatal(response)
	}
	if got := call("student", "GET", p+"/file", nil, 200); got != valid["body"] {
		t.Fatal("failed embedding lost source")
	}
	failEmbed.Store(false)
	oldDir := cfg.UploadDir
	cfg.UploadDir = filepath.Join(oldDir, "not-directory")
	os.WriteFile(cfg.UploadDir, []byte("fixture"), 0600)
	valid["version"] = 3
	valid["body"] = "这个失败写入不应保存"
	call("teacher", "PUT", p, valid, 503)
	cfg.UploadDir = oldDir
	if got := call("student", "GET", p, nil, 200); strings.Contains(got, "这个失败写入") {
		t.Fatal("partial file failure saved")
	}
	call("student", "DELETE", p, map[string]int{"version": 3}, 403)
	call("other", "DELETE", p, map[string]int{"version": 3}, 404)
	call("teacher", "DELETE", p, map[string]int{"version": 2}, 409)
	call("teacher", "DELETE", p, map[string]int{"version": 3}, 204)
	call("student", "GET", p, nil, 404)
	call("student", "GET", p+"/file", nil, 404)
	for _, table := range []string{"materials", "knowledge_entries", "knowledge_chunks", "tutor_agent_materials"} {
		col := "material_id"
		if table == "materials" {
			col = "id"
		}
		var n int
		d.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE `+col+`=?`, mid).Scan(&n)
		if n != 0 {
			t.Fatal("leftovers", table, n)
		}
	}
	mu.Lock()
	for _, id := range points {
		if id == mid {
			t.Error("vector not cleared")
		}
	}
	mu.Unlock()
	if _, err = ix.IndexMaterial(context.Background(), mid, 1, 1, "STALE_DELETED", "auto", chunking.Options{}); err == nil {
		t.Fatal("deleted material indexed")
	}
}

func TestQueuedIndexerCannotResurrectDeletedSource(t *testing.T) {
	if os.Getenv("TUTOR_INTEGRATION") != "1" {
		t.Skip("requires compose DB")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	d, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err = db.Migrate(d); err != nil {
		t.Fatal(err)
	}
	r, err := d.Exec(`INSERT INTO materials(class_id,title,file_path,uploaded_by) VALUES(1,?,'',1)`, fmt.Sprintf("queued-index-%d", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	id, _ := r.LastInsertId()
	defer d.Exec(`DELETE FROM materials WHERE id=?`, id)
	r, err = d.Exec(`INSERT INTO knowledge_entries(material_id,class_id,body_text) VALUES(?,1,'旧正文')`, id)
	if err != nil {
		t.Fatal(err)
	}
	eid, _ := r.LastInsertId()
	defer d.Exec(`DELETE FROM knowledge_entries WHERE material_id=?`, id)
	defer d.Exec(`DELETE FROM knowledge_chunks WHERE material_id=?`, id)
	ix := &retrieval.Indexer{DB: d, Cfg: cfg}
	queued := make(chan struct{})
	done := make(chan error, 1)
	err = ix.WithMaterial(context.Background(), id, func() error {
		go func() {
			close(queued)
			_, e := ix.IndexMaterial(context.Background(), id, eid, 1, "旧正文", "auto", chunking.Options{})
			done <- e
		}()
		<-queued
		select {
		case <-done:
			return fmt.Errorf("indexer did not wait for mutation lock")
		default:
		}
		if _, e := d.Exec(`DELETE FROM knowledge_entries WHERE material_id=?`, id); e != nil {
			return e
		}
		_, e := d.Exec(`DELETE FROM materials WHERE id=?`, id)
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		if err == nil {
			t.Fatal("deleted source indexed")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("queued indexer blocked")
	}
	var n int
	d.QueryRow(`SELECT COUNT(*) FROM knowledge_chunks WHERE material_id=?`, id).Scan(&n)
	if n != 0 {
		t.Fatal("deleted chunks resurrected")
	}
}
