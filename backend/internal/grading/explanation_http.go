package grading

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// @api {"method":"POST","path":"/api/submissions/{id}/explanation","summary":"为本人当前错题生成讲解","auth":"student","status":200,"request":{"type":"object","additionalProperties":false},"response":{"type":"object","required":["explanation"],"additionalProperties":false,"properties":{"explanation":{"type":"string","minLength":1,"maxLength":1500}}},"errors":{"400":"请求参数非法","401":"需要登录","403":"仅学生可用","404":"资源不存在","409":"错题已更新，请刷新","413":"请求过大","415":"请使用application/json","429":"讲解正在生成，请稍后重试","503":"讲解暂时无法生成，请稍后重试"}}
func (s *Service) Explanation(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "student")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var input map[string]json.RawMessage
	if !decode(w, r, &input) {
		return
	}
	if input == nil || len(input) != 0 {
		write(w, 400, map[string]string{"error": "请求参数非法"})
		return
	}
	work, err := s.Store.Mistake(r.Context(), u.ClassID, u.ID, id)
	if err != nil {
		failure(w, err)
		return
	}
	s.explanationMu.Lock()
	if s.explanations == nil {
		s.explanations = make(map[int64]bool)
	}
	if s.explanations[u.ID] {
		s.explanationMu.Unlock()
		w.Header().Set("Retry-After", "2")
		write(w, 429, map[string]string{"error": "讲解正在生成，请稍后重试"})
		return
	}
	s.explanations[u.ID] = true
	s.explanationMu.Unlock()
	defer func() { s.explanationMu.Lock(); delete(s.explanations, u.ID); s.explanationMu.Unlock() }()
	start := time.Now()
	budget := s.Model.Timeout
	if budget > time.Minute {
		budget = time.Minute
	}
	ctx, cancel := context.WithTimeout(r.Context(), budget)
	defer cancel()
	text, usage, modelFailure := Explain(ctx, s.Model, work)
	status := "succeeded"
	if modelFailure != nil {
		status = "failed"
	}
	if r.Context().Err() != nil {
		status = "cancelled"
	}
	defer func() {
		event("grading", "explanation", map[string]any{"actor_id": u.ID, "submission_id": id, "skill_id": ExplanationVersion, "status": status, "duration_ms": time.Since(start).Milliseconds(), "usage": usage, "model": s.Model.Model})
	}()
	if r.Context().Err() != nil {
		return
	}
	if modelFailure != nil {
		write(w, 503, map[string]string{"error": "讲解暂时无法生成，请稍后重试"})
		return
	}
	if _, err = s.Store.Mistake(r.Context(), u.ClassID, u.ID, id); err != nil {
		status = "stale"
		if err == ErrNotFound {
			write(w, 409, map[string]string{"error": "错题已更新，请刷新"})
		} else {
			failure(w, err)
		}
		return
	}
	write(w, 200, map[string]string{"explanation": text})
}
