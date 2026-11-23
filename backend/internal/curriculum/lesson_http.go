package curriculum

import "net/http"

// @api {"method":"POST","path":"/api/lesson-plans/generate","summary":"教师依据指定本班讲义生成私有教案草稿","auth":"teacher","status":200,"request":{"$ref":"#/components/schemas/LessonPlanInput"},"response":{"type":"object","required":["status","draft"],"properties":{"status":{"type":"string","enum":["generated","insufficient_evidence"]},"message":{"type":"string"},"draft":{"allOf":[{"$ref":"#/components/schemas/LessonPlan"}],"nullable":true}}},"errors":{"400":"输入非法","401":"需要登录","403":"仅教师可用","404":"资料不存在","409":"请求键冲突或来源更新","413":"请求过大","415":"类型非法","429":"正在生成","503":"生成暂不可用"}}
// @api {"method":"GET","path":"/api/lesson-plans","summary":"分页查询本人本班私有教案","auth":"teacher","status":200,"response":{"type":"object","required":["items","page","page_size"],"properties":{"items":{"type":"array","items":{"$ref":"#/components/schemas/LessonPlanSummary"}},"page":{"type":"integer"},"page_size":{"type":"integer"}}},"parameters":[{"name":"subject","in":"query","schema":{"type":"string","enum":["chinese","math","english","physics","chemistry","biology","politics","history","geography"],"default":"math"}},{"name":"q","in":"query","schema":{"type":"string","maxLength":100}},{"name":"page","in":"query","schema":{"type":"integer","minimum":1,"default":1}},{"name":"page_size","in":"query","schema":{"type":"integer","minimum":1,"maximum":100,"default":20}}],"errors":{"400":"筛选非法","401":"需要登录","403":"仅教师可用","503":"服务暂不可用"}}
// @api {"method":"GET","path":"/api/lesson-plans/{id}","summary":"读取本人私有教案及可用出处","auth":"teacher","status":200,"response":{"$ref":"#/components/schemas/LessonPlan"},"errors":{"400":"标识非法","401":"需要登录","403":"仅教师可用","404":"教案不存在","503":"服务暂不可用"}}
// @api {"method":"PUT","path":"/api/lesson-plans/{id}","summary":"按版本保存本人教案正文","auth":"teacher","status":200,"request":{"$ref":"#/components/schemas/LessonPlanUpdate"},"response":{"$ref":"#/components/schemas/LessonPlan"},"errors":{"400":"输入非法","401":"需要登录","403":"仅教师可用","404":"教案不存在","409":"版本冲突","413":"请求过大","415":"类型非法","503":"服务暂不可用"}}
// @api {"method":"DELETE","path":"/api/lesson-plans/{id}","summary":"按版本删除本人教案草稿","auth":"teacher","status":204,"request":{"type":"object","required":["expected_version"],"additionalProperties":false,"properties":{"expected_version":{"type":"integer","minimum":1}}},"errors":{"400":"输入非法","401":"需要登录","403":"仅教师可用","404":"教案不存在","409":"版本冲突","413":"请求过大","415":"类型非法","503":"服务暂不可用"}}
func mountLessons(mux *http.ServeMux, s *Service, protect func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("POST /api/lesson-plans/generate", protect(s.GenerateLessonHTTP))
	mux.HandleFunc("GET /api/lesson-plans", protect(s.ListLessonsHTTP))
	mux.HandleFunc("GET /api/lesson-plans/{id}", protect(s.GetLessonHTTP))
	mux.HandleFunc("PUT /api/lesson-plans/{id}", protect(s.UpdateLessonHTTP))
	mux.HandleFunc("DELETE /api/lesson-plans/{id}", protect(s.DeleteLessonHTTP))
}
func (s *Service) GenerateLessonHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	var in LessonInput
	if !decode(w, r, &in) {
		return
	}
	if err := in.Validate(); err != nil {
		failure(w, err)
		return
	}
	out, err := s.GenerateLesson(r.Context(), u, in)
	if r.Context().Err() != nil {
		return
	}
	if err != nil {
		failure(w, err)
		return
	}
	write(w, 200, out)
}
func (s *Service) ListLessonsHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	f, err := pointFilter(r)
	if err != nil || r.URL.Query().Has("favorites") {
		failure(w, ErrInvalid)
		return
	}
	items, err := s.Store.Lessons(r.Context(), u, f)
	if err != nil {
		failure(w, err)
		return
	}
	write(w, 200, map[string]any{"items": items, "page": f.Page, "page_size": f.Size})
}
func (s *Service) GetLessonHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	p, err := s.Store.Lesson(r.Context(), u, id)
	if err != nil {
		failure(w, err)
		return
	}
	write(w, 200, p)
}
func (s *Service) UpdateLessonHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in LessonUpdate
	if !decode(w, r, &in) {
		return
	}
	if err := in.Validate(); err != nil {
		failure(w, err)
		return
	}
	p, err := s.Store.UpdateLesson(r.Context(), u, id, in, false)
	if err != nil {
		failure(w, err)
		return
	}
	write(w, 200, p)
}
func (s *Service) DeleteLessonHTTP(w http.ResponseWriter, r *http.Request) {
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
	if _, err := s.Store.UpdateLesson(r.Context(), u, id, LessonUpdate{ExpectedVersion: in.Version}, true); err != nil {
		failure(w, err)
		return
	}
	write(w, 204, nil)
}
