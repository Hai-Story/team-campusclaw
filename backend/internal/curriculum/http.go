package curriculum

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"campusclaw/backend/internal/auth"
)

type Service struct {
	Store      *Store
	Searcher   EvidenceSearcher
	Generator  Generator
	ModelName  string
	genMu      sync.Mutex
	generating map[int64]bool
}

func write(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if status != 204 {
		_ = json.NewEncoder(w).Encode(data)
	}
}
func failure(w http.ResponseWriter, err error) {
	status, text := 503, "服务暂时不可用，请稍后重试"
	switch {
	case errors.Is(err, ErrNotFound):
		status, text = 404, ErrNotFound.Error()
	case errors.Is(err, ErrInvalid):
		status, text = 400, ErrInvalid.Error()
	case errors.Is(err, ErrConflict):
		status, text = 409, ErrConflict.Error()
	case errors.Is(err, ErrEvidence):
		status, text = 409, ErrEvidence.Error()
	case errors.Is(err, ErrBusy):
		status, text = 429, ErrBusy.Error()
		w.Header().Set("Retry-After", "2")
	}
	write(w, status, map[string]string{"error": text})
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
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		failure(w, ErrInvalid)
		return 0, false
	}
	return id, true
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		write(w, 415, map[string]string{"error": "请使用application/json"})
		return false
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10))
	d.DisallowUnknownFields()
	err = d.Decode(v)
	if err == nil {
		var extra any
		if d.Decode(&extra) != io.EOF {
			err = ErrInvalid
		}
	}
	if err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			write(w, 413, map[string]string{"error": "请求内容过大"})
		} else {
			failure(w, ErrInvalid)
		}
		return false
	}
	return true
}
func pointFilter(r *http.Request) (PointFilter, error) {
	f := PointFilter{Subject: "math", Page: 1, Size: 20}
	for k, values := range r.URL.Query() {
		if len(values) != 1 {
			return f, ErrInvalid
		}
		v := values[0]
		switch k {
		case "subject":
			if !ValidSubject(v) {
				return f, ErrInvalid
			}
			f.Subject = v
		case "q":
			f.Q = strings.TrimSpace(v)
			if len([]rune(f.Q)) > 100 {
				return f, ErrInvalid
			}
		case "favorites":
			if v != "true" && v != "false" {
				return f, ErrInvalid
			}
			f.Favorites = v == "true"
		case "page", "page_size":
			n, err := strconv.ParseInt(v, 10, 32)
			if err != nil || n < 1 {
				return f, ErrInvalid
			}
			if k == "page" {
				f.Page = int(n)
			} else {
				f.Size = int(n)
				if f.Size > 100 {
					f.Size = 100
				}
			}
		default:
			return f, ErrInvalid
		}
	}
	return f, nil
}

// @api {"method":"GET","path":"/api/knowledge-points","summary":"分页查询本班知识点与本人收藏","auth":"login","status":200,"response":{"type":"object","required":["items","page","page_size"],"properties":{"items":{"type":"array","items":{"$ref":"#/components/schemas/KnowledgePoint"}},"page":{"type":"integer"},"page_size":{"type":"integer"}}},"parameters":[{"name":"subject","in":"query","schema":{"type":"string","enum":["chinese","math","english","science","morality"],"default":"math"}},{"name":"q","in":"query","schema":{"type":"string","maxLength":100}},{"name":"favorites","in":"query","schema":{"type":"boolean","default":false}},{"name":"page","in":"query","schema":{"type":"integer","minimum":1,"default":1}},{"name":"page_size","in":"query","schema":{"type":"integer","minimum":1,"maximum":100,"default":20}}],"errors":{"400":"筛选或分页非法","401":"需要登录","503":"服务暂时不可用"}}
// @api {"method":"POST","path":"/api/knowledge-points","summary":"创建本班知识点及资源关联","auth":"teacher","status":201,"request":{"$ref":"#/components/schemas/KnowledgePointInput"},"response":{"$ref":"#/components/schemas/KnowledgePoint"},"errors":{"400":"输入非法","401":"需要登录","403":"仅教师可写","404":"关联资源不存在或无权","409":"名称重复","413":"请求过大","415":"类型非法","503":"服务暂时不可用"}}
// @api {"method":"GET","path":"/api/knowledge-points/{id}","summary":"读取本班知识点详情","auth":"login","status":200,"response":{"$ref":"#/components/schemas/KnowledgePoint"},"errors":{"400":"标识非法","401":"需要登录","404":"资源不存在","503":"服务暂时不可用"}}
// @api {"method":"PUT","path":"/api/knowledge-points/{id}","summary":"按版本更新知识点与关联","auth":"teacher","status":200,"request":{"$ref":"#/components/schemas/KnowledgePointUpdate"},"response":{"$ref":"#/components/schemas/KnowledgePoint"},"errors":{"400":"输入非法","401":"需要登录","403":"仅教师可写","404":"资源不存在","409":"版本冲突或名称重复","413":"请求过大","415":"类型非法","503":"服务暂时不可用"}}
// @api {"method":"DELETE","path":"/api/knowledge-points/{id}","summary":"按版本删除知识点，保留原资料与作业","auth":"teacher","status":204,"request":{"type":"object","required":["expected_version"],"additionalProperties":false,"properties":{"expected_version":{"type":"integer","minimum":1}}},"errors":{"400":"输入非法","401":"需要登录","403":"仅教师可写","404":"资源不存在","409":"版本冲突","413":"请求过大","415":"类型非法","503":"服务暂时不可用"}}
// @api {"method":"PUT","path":"/api/knowledge-points/{id}/favorite","summary":"幂等收藏本班知识点","auth":"student","status":204,"errors":{"400":"标识非法","401":"需要登录","403":"仅学生可收藏","404":"资源不存在","503":"服务暂时不可用"}}
// @api {"method":"DELETE","path":"/api/knowledge-points/{id}/favorite","summary":"幂等取消本人收藏","auth":"student","status":204,"errors":{"400":"标识非法","401":"需要登录","403":"仅学生可收藏","404":"资源不存在","503":"服务暂时不可用"}}
func Mount(mux *http.ServeMux, s *Service, protect func(http.HandlerFunc) http.HandlerFunc) {
	mountLessons(mux, s, protect)
	mux.HandleFunc("GET /api/knowledge-points", protect(s.ListPoints))
	mux.HandleFunc("POST /api/knowledge-points", protect(s.SavePoint))
	mux.HandleFunc("GET /api/knowledge-points/{id}", protect(s.GetPoint))
	mux.HandleFunc("PUT /api/knowledge-points/{id}", protect(s.SavePoint))
	mux.HandleFunc("DELETE /api/knowledge-points/{id}", protect(s.DeletePoint))
	mux.HandleFunc("PUT /api/knowledge-points/{id}/favorite", protect(s.Favorite))
	mux.HandleFunc("DELETE /api/knowledge-points/{id}/favorite", protect(s.Favorite))
}
func (s *Service) ListPoints(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "")
	if !ok {
		return
	}
	f, err := pointFilter(r)
	if err != nil {
		failure(w, err)
		return
	}
	items, err := s.Store.Points(r.Context(), u, f)
	if err != nil {
		failure(w, err)
		return
	}
	write(w, 200, map[string]any{"items": items, "page": f.Page, "page_size": f.Size})
}
func (s *Service) GetPoint(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	p, err := s.Store.Point(r.Context(), u, id)
	if err != nil {
		failure(w, err)
		return
	}
	write(w, 200, p)
}
func (s *Service) SavePoint(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	var id int64
	status := 201
	if r.Method == "PUT" {
		id, ok = pathID(w, r)
		if !ok {
			return
		}
		status = 200
	}
	var in PointInput
	if !decode(w, r, &in) {
		return
	}
	if err := in.Validate(id > 0); err != nil {
		failure(w, err)
		return
	}
	p, err := s.Store.SavePoint(r.Context(), u, id, in)
	if err != nil {
		failure(w, err)
		return
	}
	write(w, status, p)
}
func (s *Service) DeletePoint(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in struct {
		Version int `json:"expected_version"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Version < 1 {
		failure(w, ErrInvalid)
		return
	}
	if err := s.Store.DeletePoint(r.Context(), u, id, in.Version); err != nil {
		failure(w, err)
		return
	}
	write(w, 204, nil)
}
func (s *Service) Favorite(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "student")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.Store.Favorite(r.Context(), u, id, r.Method == "PUT"); err != nil {
		failure(w, err)
		return
	}
	write(w, 204, nil)
}
