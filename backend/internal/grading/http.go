package grading

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"campusclaw/backend/internal/auth"
)

type Service struct {
	Store         *Store
	Model         Settings
	explanationMu sync.Mutex
	explanations  map[int64]bool
}

func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
func failure(w http.ResponseWriter, err error) {
	status := 503
	message := "服务暂时不可用，请稍后重试"
	if errors.Is(err, ErrNotFound) {
		status = 404
		message = "资源不存在"
	}
	if errors.Is(err, ErrConflict) {
		status = 409
		message = "该提交标识已用于其他内容，请使用新的提交标识"
	}
	write(w, status, map[string]string{"error": message})
}
func actor(w http.ResponseWriter, r *http.Request, role string) (auth.User, bool) {
	u, ok := auth.FromContext(r.Context())
	if !ok {
		write(w, 401, map[string]string{"error": "需要登录"})
		return u, false
	}
	if role != "" && u.Role != role {
		write(w, 403, map[string]string{"error": "当前角色无权执行此操作"})
		return u, false
	}
	return u, true
}
func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		write(w, 415, map[string]string{"error": "请使用application/json"})
		return false
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536))
	d.DisallowUnknownFields()
	err := d.Decode(value)
	if err == nil {
		var extra any
		if d.Decode(&extra) != io.EOF {
			err = errors.New("extra body")
		}
	}
	if err != nil {
		status := 400
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			status = 413
		}
		write(w, status, map[string]string{"error": "请求内容格式或大小不合法"})
		return false
	}
	return true
}
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		write(w, 400, map[string]string{"error": "标识须为正整数"})
		return 0, false
	}
	return id, true
}
func pageArgs(r *http.Request) (int, int, error) {
	page, size := 1, 20
	for k, values := range r.URL.Query() {
		if k == "subject" && len(values) == 1 && ValidSubject(values[0]) {
			continue
		}
		if len(values) != 1 || (k != "page" && k != "page_size") {
			return 0, 0, errors.New("invalid query")
		}
		n, e := strconv.Atoi(values[0])
		if e != nil || n < 1 || n > 2147483647 {
			return 0, 0, errors.New("invalid page")
		}
		if k == "page" {
			page = n
		} else {
			size = n
		}
	}
	if size > 100 {
		size = 100
	}
	return page, size, nil
}

func Mount(mux *http.ServeMux, s *Service, protect func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /api/assignments/{id}/submissions", protect(s.ListSubmissions))
	mux.HandleFunc("GET /api/assignments/{id}/my-submission", protect(s.MySubmission))
	mux.HandleFunc("GET /api/mistakes", protect(s.Mistakes))
	mux.HandleFunc("POST /api/submissions/{id}/explanation", protect(s.Explanation))
	mux.HandleFunc("PUT /api/submissions/{id}/review", protect(s.Review))
	mux.HandleFunc("POST /api/assignments", protect(s.CreateAssignment))
	mux.HandleFunc("GET /api/assignments", protect(s.ListAssignments))
	mux.HandleFunc("GET /api/assignments/{id}", protect(s.GetAssignment))
	mux.HandleFunc("POST /api/assignments/{id}/submissions", protect(s.Submit))
	mux.HandleFunc("GET /api/tasks/{id}", protect(s.GetTask))
	mux.HandleFunc("GET /api/submissions/{id}", protect(s.GetSubmission))
}

// @api {"method":"POST","path":"/api/assignments","summary":"发布本班客观题作业","auth":"teacher","status":201,"request":{"type":"object","required":["title","question","expected_answer","max_score","subject"],"properties":{"title":{"type":"string","maxLength":120,"minLength":1},"question":{"type":"string","maxLength":4000,"minLength":1},"expected_answer":{"type":"string","maxLength":2000,"minLength":1},"max_score":{"type":"integer","minimum":1,"maximum":100},"subject":{"type":"string","enum":["chinese","math","english","science","morality"]}},"additionalProperties":false},"response":{"$ref":"#/components/schemas/GradingAssignment"},"errors":{"400":"参数非法","401":"需要登录","403":"仅教师可发布","503":"服务暂时不可用","413":"请求过大","415":"请使用application/json"}}
func (s *Service) CreateAssignment(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	var input AssignmentInput
	if !decode(w, r, &input) {
		return
	}
	if err := input.Validate(); err != nil {
		write(w, 400, map[string]string{"error": err.Error()})
		return
	}
	a, err := s.Store.CreateAssignment(r.Context(), u.ClassID, u.ID, input)
	if err != nil {
		failure(w, err)
		return
	}
	write(w, 201, a)
}

// @api {"method":"GET","path":"/api/assignments","summary":"分页读取本班作业","auth":"login","status":200,"response":{"type":"object","required":["items","page","page_size"],"properties":{"items":{"type":"array","items":{"$ref":"#/components/schemas/GradingAssignment"}},"page":{"type":"integer"},"page_size":{"type":"integer","minimum":1,"maximum":100}}},"errors":{"400":"分页参数非法","401":"需要登录","503":"服务暂时不可用"},"parameters":[{"name":"page","in":"query","schema":{"type":"integer","minimum":1,"default":1}},{"name":"page_size","in":"query","schema":{"type":"integer","minimum":1,"default":20},"description":"超过100按100返回"},{"name":"subject","in":"query","schema":{"type":"string","enum":["chinese","math","english","science","morality"],"default":"math"}}]}
func (s *Service) ListAssignments(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "")
	if !ok {
		return
	}
	page, size, err := pageArgs(r)
	if err != nil {
		write(w, 400, map[string]string{"error": "分页参数非法"})
		return
	}
	subject := r.URL.Query().Get("subject")
	if subject == "" {
		subject = "math"
	}
	items, err := s.Store.Assignments(r.Context(), u.ClassID, subject, page, size)
	if err != nil {
		failure(w, err)
		return
	}
	if u.Role != "teacher" {
		for i := range items {
			items[i].ExpectedAnswer = ""
		}
	}
	write(w, 200, map[string]any{"items": items, "page": page, "page_size": size})
}

// @api {"method":"GET","path":"/api/assignments/{id}","summary":"读取本班作业，学生不返回标准答案","auth":"login","status":200,"response":{"$ref":"#/components/schemas/GradingAssignment"},"errors":{"400":"标识非法","401":"需要登录","404":"资源不存在","503":"服务暂时不可用"}}
func (s *Service) GetAssignment(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	a, err := s.Store.Assignment(r.Context(), u.ClassID, id)
	if err != nil {
		failure(w, err)
		return
	}
	if u.Role != "teacher" {
		a.ExpectedAnswer = ""
	}
	write(w, 200, a)
}

// @api {"method":"POST","path":"/api/assignments/{id}/submissions","summary":"原子受理提交与异步批改任务","auth":"student","status":202,"parameters":[{"name":"Idempotency-Key","in":"header","required":true,"schema":{"type":"string","pattern":"^[A-Za-z0-9-]{1,64}$"}}],"request":{"type":"object","required":["answer"],"properties":{"answer":{"type":"string","minLength":1,"maxLength":10000}},"additionalProperties":false},"response":{"$ref":"#/components/schemas/SubmissionReceipt"},"errors":{"400":"答案或提交标识非法","401":"需要登录","403":"仅学生可提交","404":"资源不存在","409":"同键内容冲突","413":"请求过大","415":"请使用application/json","503":"服务暂时不可用"}}
func (s *Service) Submit(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "student")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var input struct {
		Answer string `json:"answer"`
	}
	if !decode(w, r, &input) {
		return
	}
	answer := strings.TrimSpace(input.Answer)
	if n := len([]rune(answer)); n < 1 || n > 10000 || !keyPattern.MatchString(r.Header.Get("Idempotency-Key")) {
		write(w, 400, map[string]string{"error": "答案须为1至10000字，提交标识须为1至64个字母、数字或连字符"})
		return
	}
	out, err := s.Store.Submit(r.Context(), u.ClassID, u.ID, id, r.Header.Get("Idempotency-Key"), answer)
	if err != nil {
		failure(w, err)
		return
	}
	write(w, 202, out)
}

// @api {"method":"GET","path":"/api/tasks/{id}","summary":"只读查询本人的任务或教师本班任务","auth":"login","status":200,"response":{"$ref":"#/components/schemas/GradingTask"},"errors":{"400":"任务标识非法","401":"需要登录","404":"资源不存在","503":"服务暂时不可用"}}
func (s *Service) GetTask(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "")
	if !ok {
		return
	}
	id := r.PathValue("id")
	if !taskPattern.MatchString(id) {
		write(w, 400, map[string]string{"error": "任务标识格式不合法"})
		return
	}
	task, err := s.Store.Task(r.Context(), u.ClassID, u.ID, u.Role == "teacher", id, 0)
	if err != nil {
		failure(w, err)
		return
	}
	write(w, 200, task)
}

// @api {"method":"GET","path":"/api/submissions/{id}","summary":"只读查询授权提交的批改任务","auth":"login","status":200,"response":{"$ref":"#/components/schemas/GradingTask"},"errors":{"400":"标识非法","401":"需要登录","404":"资源不存在","503":"服务暂时不可用"}}
func (s *Service) GetSubmission(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	task, err := s.Store.Task(r.Context(), u.ClassID, u.ID, u.Role == "teacher", "", id)
	if err != nil {
		failure(w, err)
		return
	}
	write(w, 200, task)
}
