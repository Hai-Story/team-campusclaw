package securityaudit

import (
	"campusclaw/backend/internal/auditlog"
	"campusclaw/backend/internal/auth"
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

type Service struct{ Store *auditlog.Store }

func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int) {
	text := "筛选参数无效，请检查后重试"
	if status == 503 {
		text = "审计记录读取失败，请稍后重试"
	}
	if status == 404 {
		text = "资源不存在"
	}
	write(w, status, map[string]string{"error": text})
}

var skillPattern = regexp.MustCompile(`^[a-zA-Z0-9_.:-]{1,80}$`)
var requestPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

func parse(r *http.Request, now time.Time) (auditlog.Filter, bool) {
	f := auditlog.Filter{Page: 1, Size: 20, From: now.Add(-24 * time.Hour), To: now}
	for key, vs := range r.URL.Query() {
		if len(vs) != 1 || vs[0] == "" {
			return f, false
		}
		v := vs[0]
		switch key {
		case "page", "page_size", "actor_id", "before_id":
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil || n < 1 || n > 2147483647 {
				return f, false
			}
			switch key {
			case "page":
				f.Page = int(n)
			case "page_size":
				f.Size = int(n)
			case "actor_id":
				f.ActorID = n
			case "before_id":
				f.BeforeID = n
			}
		case "skill_id":
			if !skillPattern.MatchString(v) {
				return f, false
			}
			f.Skill = v
		case "action":
			if auditlog.Safe(v) == "unknown" {
				return f, false
			}
			f.Action = v
		case "result":
			switch v {
			case "started", "succeeded", "failed", "denied", "cancelled", "pending", "discarded":
				f.Result = v
			default:
				return f, false
			}
		case "request_id":
			if !requestPattern.MatchString(v) {
				return f, false
			}
			f.Request = v
		case "from", "to":
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				return f, false
			}
			if key == "from" {
				f.From = t
			} else {
				f.To = t
			}
		default:
			return f, false
		}
	}
	if f.Size > 100 {
		f.Size = 100
	}
	if f.To.Before(f.From) || f.To.Sub(f.From) > 31*24*time.Hour {
		return f, false
	}
	return f, true
}

// @api {"method":"GET","path":"/api/audit-events","summary":"分页查询本班操作审计","auth":"teacher","status":200,"parameters":[{"name":"page","in":"query","schema":{"type":"integer","minimum":1,"default":1}},{"name":"page_size","in":"query","schema":{"type":"integer","minimum":1,"maximum":100,"default":20}},{"name":"actor_id","in":"query","schema":{"type":"integer","minimum":1}},{"name":"skill_id","in":"query","schema":{"type":"string","maxLength":80}},{"name":"from","in":"query","schema":{"type":"string","format":"date-time"}},{"name":"to","in":"query","schema":{"type":"string","format":"date-time"}},{"name":"result","in":"query","schema":{"type":"string"}},{"name":"action","in":"query","schema":{"type":"string"}},{"name":"request_id","in":"query","schema":{"type":"string"}},{"name":"before_id","in":"query","schema":{"type":"integer","minimum":1}}],"response":{"type":"object","required":["items","page","page_size","total","snapshot_id"],"properties":{"items":{"type":"array","items":{"type":"object"}},"page":{"type":"integer"},"page_size":{"type":"integer"},"total":{"type":"integer"},"snapshot_id":{"type":"integer"}}},"errors":{"400":"筛选参数无效","401":"未登录","403":"仅教师可查","503":"审计服务不可用"},"description":"默认最近24小时，最大31天；按id降序。首次返回snapshot_id，后续分页以before_id固定快照上界。记录含actor_id、time、action、object、result、skill_id、model.usage与duration_ms；无模型时model和skill_id为null。"}
func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromContext(r.Context())
	f, ok := parse(r, time.Now().UTC())
	if !ok {
		fail(w, 400)
		return
	}
	f.ClassID = u.ClassID
	if f.BeforeID == 0 {
		if trace := auditlog.From(r.Context()); trace != nil {
			f.BeforeID = trace.ID() - 1
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	items, total, snapshot, err := s.Store.List(ctx, f)
	if err != nil {
		fail(w, 503)
		return
	}
	write(w, 200, map[string]any{"items": items, "total": total, "page": f.Page, "page_size": f.Size, "snapshot_id": snapshot})
}

// @api {"method":"GET","path":"/api/audit-events/{id}","summary":"读取本班操作审计详情","auth":"teacher","status":200,"response":{"type":"object"},"errors":{"400":"标识或参数无效","401":"未登录","403":"仅教师可查","404":"资源不存在","503":"审计服务不可用"}}
func (s *Service) Detail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 || r.URL.RawQuery != "" {
		fail(w, 400)
		return
	}
	u, _ := auth.FromContext(r.Context())
	v, err := s.Store.Get(r.Context(), u.ClassID, id)
	if auditlog.NotFound(err) {
		fail(w, 404)
		return
	}
	if err != nil {
		fail(w, 503)
		return
	}
	write(w, 200, v)
}
func Mount(mux *http.ServeMux, s *Service, authenticated func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /api/audit-events", authenticated(auth.RequireTeacher(s.List)))
	mux.HandleFunc("GET /api/audit-events/{id}", authenticated(auth.RequireTeacher(s.Detail)))
}
