package curriculum

import (
	"net/http"
)

// @api {"method":"POST","path":"/api/paper-drafts/{id}/publish","summary":"将私有试卷指定版本发布为不可变班级作业","auth":"teacher","status":201,"response":{"$ref":"#/components/schemas/PublishedPaper"},"errors":{"400":"输入非法","401":"需要登录","403":"角色不允许","404":"资源不存在","409":"版本、来源或请求键冲突","413":"请求过大","415":"类型非法","503":"服务暂不可用"},"request":{"$ref":"#/components/schemas/ExpectedVersion"}}
// @api {"method":"GET","path":"/api/published-papers","summary":"分页读取本班多题作业","auth":"login","status":200,"response":{"$ref":"#/components/schemas/PublishedPage"},"errors":{"400":"输入非法","401":"需要登录","403":"角色不允许","404":"资源不存在","409":"版本、来源或请求键冲突","413":"请求过大","415":"类型非法","503":"服务暂不可用"}}
// @api {"method":"GET","path":"/api/published-papers/{id}","summary":"读取本班试卷，学生不含参考与解析","auth":"login","status":200,"response":{"$ref":"#/components/schemas/PublishedPaper"},"errors":{"400":"输入非法","401":"需要登录","403":"角色不允许","404":"资源不存在","409":"版本、来源或请求键冲突","413":"请求过大","415":"类型非法","503":"服务暂不可用"}}
// @api {"method":"POST","path":"/api/published-papers/{id}/attempts","summary":"原子整卷提交并复用逐题初判任务","auth":"student","status":202,"response":{"$ref":"#/components/schemas/PaperAttempt"},"errors":{"400":"输入非法","401":"需要登录","403":"角色不允许","404":"资源不存在","409":"版本、来源或请求键冲突","413":"请求过大","415":"类型非法","503":"服务暂不可用"},"request":{"$ref":"#/components/schemas/PaperAttemptInput"},"parameters":[{"name":"Idempotency-Key","in":"header","required":true,"schema":{"type":"string","pattern":"^[A-Za-z0-9-]{1,64}$"}}]}
// @api {"method":"GET","path":"/api/published-papers/{id}/attempts","summary":"教师读取本班最新提交或学生本人历史","auth":"login","status":200,"response":{"$ref":"#/components/schemas/PaperAttemptPage"},"errors":{"400":"输入非法","401":"需要登录","403":"角色不允许","404":"资源不存在","409":"版本、来源或请求键冲突","413":"请求过大","415":"类型非法","503":"服务暂不可用"}}
// @api {"method":"GET","path":"/api/paper-attempts/{id}","summary":"读取授权整卷逐题作答及教师终评汇总","auth":"login","status":200,"response":{"$ref":"#/components/schemas/PaperAttempt"},"errors":{"400":"输入非法","401":"需要登录","403":"角色不允许","404":"资源不存在","409":"版本、来源或请求键冲突","413":"请求过大","415":"类型非法","503":"服务暂不可用"}}
func mountPublished(mux *http.ServeMux, s *Service, protect func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("POST /api/paper-drafts/{id}/publish", protect(s.PublishPaperHTTP))
	mux.HandleFunc("GET /api/published-papers", protect(s.ListPublishedHTTP))
	mux.HandleFunc("GET /api/published-papers/{id}", protect(s.GetPublishedHTTP))
	mux.HandleFunc("POST /api/published-papers/{id}/attempts", protect(s.SubmitPaperHTTP))
	mux.HandleFunc("GET /api/published-papers/{id}/attempts", protect(s.ListPaperAttemptsHTTP))
	mux.HandleFunc("GET /api/paper-attempts/{id}", protect(s.GetPaperAttemptHTTP))
}
func (s *Service) PublishPaperHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in ExpectedVersion
	if !decode(w, r, &in) {
		return
	}
	if in.ExpectedVersion < 1 {
		failure(w, ErrInvalid)
		return
	}
	v, e := s.Store.PublishPaper(r.Context(), u, id, in.ExpectedVersion)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 201, v)
}
func (s *Service) ListPublishedHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "")
	if !ok {
		return
	}
	f, e := paperFilter(r)
	if e != nil {
		failure(w, e)
		return
	}
	v, e := s.Store.PublishedPapers(r.Context(), u, f)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, map[string]any{"items": v, "page": f.Page, "page_size": f.Size})
}
func paperFilter(r *http.Request) (PointFilter, error) {
	for k := range r.URL.Query() {
		if k != "subject" && k != "page" && k != "page_size" {
			return PointFilter{}, ErrInvalid
		}
	}
	return pointFilter(r)
}
func (s *Service) GetPublishedHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	v, e := s.Store.PublishedPaper(r.Context(), u, id)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Service) SubmitPaperHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "student")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if !clientKey.MatchString(key) {
		failure(w, ErrInvalid)
		return
	}
	var in PaperAttemptInput
	if !decodeLimit(w, r, &in, 1<<20) {
		return
	}
	v, e := s.Store.SubmitPaper(r.Context(), u, id, key, in)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 202, v)
}
func (s *Service) ListPaperAttemptsHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	f, e := paperFilter(r)
	if e != nil || r.URL.Query().Has("subject") {
		failure(w, ErrInvalid)
		return
	}
	v, e := s.Store.PaperAttempts(r.Context(), u, id, f.Page, f.Size)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, map[string]any{"items": v, "page": f.Page, "page_size": f.Size})
}
func (s *Service) GetPaperAttemptHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	v, e := s.Store.PaperAttempt(r.Context(), u, id)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}
