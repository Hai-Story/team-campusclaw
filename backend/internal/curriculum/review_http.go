package curriculum

import (
	"net/http"
	"strconv"
	"time"
)

func reviewFilter(r *http.Request, allowStatus bool) (PointFilter, string, error) {
	f := PointFilter{Subject: "math", Page: 1, Size: 20}
	status := "due"
	for k, vs := range r.URL.Query() {
		if len(vs) != 1 {
			return f, status, ErrInvalid
		}
		v := vs[0]
		switch k {
		case "subject":
			if !ValidSubject(v) {
				return f, status, ErrInvalid
			}
			f.Subject = v
		case "status":
			if !allowStatus || (v != "due" && v != "all") {
				return f, status, ErrInvalid
			}
			status = v
		case "page", "page_size":
			n, e := strconv.ParseInt(v, 10, 32)
			if e != nil || n < 1 {
				return f, status, ErrInvalid
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
			return f, status, ErrInvalid
		}
	}
	return f, status, nil
}

// @api {"auth":"student","status":200,"errors":{"400":"输入非法","401":"需要登录","403":"仅学生可操作","503":"复习暂时不可用","413":"正文过大","415":"类型错误"},"method":"POST","path":"/api/review-cards/sync","summary":"同步当前错题和收藏复习卡","response":{"type":"object","properties":{"status":{"type":"string"}},"required":["status"]},"request":{"type":"object","additionalProperties":false}}
// @api {"auth":"student","status":200,"errors":{"400":"输入非法","401":"需要登录","403":"仅学生可操作","503":"复习暂时不可用"},"method":"GET","path":"/api/review-cards","summary":"查询本人当前有效复习卡","response":{"$ref":"#/components/schemas/ReviewReport"},"parameters":[{"name":"subject","in":"query","schema":{"type":"string","enum":["chinese","math","english","science","morality"],"default":"math"}},{"name":"page","in":"query","schema":{"type":"integer","minimum":1,"default":1}},{"name":"page_size","in":"query","schema":{"type":"integer","minimum":1,"maximum":100,"default":20}},{"name":"status","in":"query","schema":{"type":"string","enum":["due","all"],"default":"due"}}]}
// @api {"auth":"student","status":200,"errors":{"400":"输入非法","401":"需要登录","403":"仅学生可操作","503":"复习暂时不可用"},"method":"GET","path":"/api/review-cards/count","summary":"本人到期复习提醒","response":{"type":"object","properties":{"due_count":{"type":"integer","minimum":0},"server_time":{"type":"string","format":"date-time"}},"required":["due_count","server_time"]}}
// @api {"auth":"student","status":200,"errors":{"400":"输入非法","401":"需要登录","403":"仅学生可操作","503":"复习暂时不可用"},"method":"GET","path":"/api/review-settings","summary":"读取本人复习间隔","response":{"$ref":"#/components/schemas/ReviewSettings"}}
// @api {"auth":"student","status":200,"errors":{"400":"输入非法","401":"需要登录","403":"仅学生可操作","503":"复习暂时不可用","413":"正文过大","415":"类型错误","409":"设置版本冲突"},"method":"PUT","path":"/api/review-settings","summary":"按版本设置未来复习间隔","response":{"$ref":"#/components/schemas/ReviewSettings"},"request":{"type":"object","properties":{"intervals":{"type":"array","items":{"type":"integer","minimum":1,"maximum":365},"minItems":5,"maxItems":5},"expected_version":{"type":"integer","minimum":1}},"required":["intervals","expected_version"],"additionalProperties":false}}
// @api {"auth":"student","status":200,"errors":{"400":"输入非法","401":"需要登录","403":"仅学生可操作","503":"复习暂时不可用","413":"正文过大","415":"类型错误","404":"复习卡不存在或无权","409":"版本、请求键、源状态冲突或尚未到期"},"method":"POST","path":"/api/review-cards/{id}/complete","summary":"幂等完成本人到期复习","response":{"$ref":"#/components/schemas/ReviewCompletion"},"request":{"type":"object","properties":{"feedback":{"type":"string","enum":["good","again"]},"client_key":{"type":"string","pattern":"^[A-Za-z0-9-]{1,64}$"},"expected_version":{"type":"integer","minimum":1}},"required":["feedback","client_key","expected_version"],"additionalProperties":false}}
// @api {"auth":"student","status":200,"errors":{"400":"输入非法","401":"需要登录","403":"仅学生可操作","503":"复习暂时不可用"},"method":"GET","path":"/api/review-completions","summary":"分页读取本人复习完成历史","response":{"type":"object","properties":{"items":{"type":"array","items":{"$ref":"#/components/schemas/ReviewCompletion"}},"page":{"type":"integer","minimum":1},"page_size":{"type":"integer","minimum":1}},"required":["items","page","page_size"]},"parameters":[{"name":"subject","in":"query","schema":{"type":"string","enum":["chinese","math","english","science","morality"],"default":"math"}},{"name":"page","in":"query","schema":{"type":"integer","minimum":1,"default":1}},{"name":"page_size","in":"query","schema":{"type":"integer","minimum":1,"maximum":100,"default":20}}]}
func mountReview(mux *http.ServeMux, s *Service, p func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("POST /api/review-cards/sync", p(s.SyncReview))
	mux.HandleFunc("GET /api/review-cards", p(s.ListReview))
	mux.HandleFunc("GET /api/review-cards/count", p(s.ReviewCount))
	mux.HandleFunc("POST /api/review-cards/{id}/complete", p(s.CompleteReview))
	mux.HandleFunc("GET /api/review-settings", p(s.GetReviewSettings))
	mux.HandleFunc("PUT /api/review-settings", p(s.SaveReviewSettings))
	mux.HandleFunc("GET /api/review-completions", p(s.ReviewHistory))
}
func (s *Service) SyncReview(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "student")
	if !ok {
		return
	}
	var in *struct{}
	if !decode(w, r, &in) {
		return
	}
	if in == nil {
		failure(w, ErrInvalid)
		return
	}
	if e := s.Store.SyncReview(r.Context(), u, time.Now()); e != nil {
		failure(w, e)
		return
	}
	write(w, 200, map[string]string{"status": "synced"})
}
func (s *Service) ListReview(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "student")
	if !ok {
		return
	}
	f, status, e := reviewFilter(r, true)
	if e != nil {
		failure(w, e)
		return
	}
	v, e := s.Store.ReviewReport(r.Context(), u, f.Subject, status, f.Page, f.Size, time.Now())
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Service) ReviewCount(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "student")
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		failure(w, ErrInvalid)
		return
	}
	now := time.Now()
	n, e := s.Store.ReviewDueCount(r.Context(), u, now)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, map[string]any{"due_count": n, "server_time": now.UTC()})
}
func (s *Service) GetReviewSettings(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "student")
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		failure(w, ErrInvalid)
		return
	}
	v, e := s.Store.ReviewSettings(r.Context(), u)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Service) SaveReviewSettings(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "student")
	if !ok {
		return
	}
	var in ReviewSettingsInput
	if !decode(w, r, &in) {
		return
	}
	if e := in.Validate(); e != nil {
		failure(w, e)
		return
	}
	v, e := s.Store.SaveReviewSettings(r.Context(), u, in)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Service) CompleteReview(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "student")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in ReviewFeedback
	if !decode(w, r, &in) {
		return
	}
	if e := in.Validate(); e != nil {
		failure(w, e)
		return
	}
	v, e := s.Store.CompleteReview(r.Context(), u, id, in, time.Now())
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Service) ReviewHistory(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "student")
	if !ok {
		return
	}
	f, _, e := reviewFilter(r, false)
	if e != nil {
		failure(w, e)
		return
	}
	v, e := s.Store.ReviewHistory(r.Context(), u, f.Subject, f.Page, f.Size)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, map[string]any{"items": v, "page": f.Page, "page_size": f.Size})
}
