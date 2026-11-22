package grading

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"time"
)

// @api {"method":"POST","path":"/api/submissions/{id}/explanation","summary":"为本人当前错题生成讲解","auth":"student","status":200,"request":{"type":"object","additionalProperties":false},"response":{"type":"object","required":["explanation","status","citations"],"additionalProperties":false,"properties":{"explanation":{"type":"string","minLength":1,"maxLength":1500},"status":{"type":"string","enum":["generated","insufficient_evidence"]},"citations":{"type":"array","items":{"type":"object","required":["index","material_id","chunk_id","excerpt"],"properties":{"index":{"type":"integer"},"material_id":{"type":"integer"},"chunk_id":{"type":"integer"},"chunk_index":{"type":"integer"},"char_start":{"type":"integer"},"char_end":{"type":"integer"},"material_title":{"type":"string"},"excerpt":{"type":"string"}}}}}},"errors":{"400":"请求参数非法","401":"需要登录","403":"仅学生可用","404":"资源不存在","409":"错题已更新，请刷新","413":"请求过大","415":"请使用application/json","429":"讲解正在生成，请稍后重试","503":"讲解暂时无法生成，请稍后重试"}}
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
	ctx, cancel := context.WithTimeout(r.Context(), 121*time.Second)
	defer cancel()
	refs := []ExplanationCitation{}
	status := "failed"
	var usage Usage
	defer func() {
		if r.Context().Err() != nil {
			status = "cancelled"
		}
		event("grading", "explanation", map[string]any{"actor_id": u.ID, "submission_id": id, "skill_id": ExplanationVersion, "status": status, "duration_ms": time.Since(start).Milliseconds(), "usage": usage, "model": s.Model.Model, "hit_count": len(refs)})
	}()
	if s.Searcher == nil {
		write(w, 503, map[string]string{"error": "资料检索暂时不可用，请稍后重试"})
		return
	}
	ids, err := s.Store.ExplanationMaterials(ctx, work)
	if err != nil {
		failure(w, err)
		return
	}
	hits, err := s.Searcher.SearchBound(ctx, u.ClassID, work.Question, ids, 4)
	if err != nil {
		write(w, 503, map[string]string{"error": "资料检索暂时不可用，请稍后重试"})
		return
	}
	refs, err = s.Store.ExplanationEvidence(ctx, work, hits)
	if err != nil {
		failure(w, err)
		return
	}
	if len(refs) == 0 {
		status = "insufficient_evidence"
		write(w, 200, ExplanationResponse{Explanation: "本班资料中未找到足够依据，请联系教师补充相关资料后重试。", Status: status, Citations: []ExplanationCitation{}})
		return
	}
	result, used, modelFailure := ExplainEvidence(ctx, s.Model, work, refs)
	usage = used
	if r.Context().Err() != nil {
		return
	}
	if modelFailure != nil {
		write(w, 503, map[string]string{"error": "讲解暂时无法生成，请稍后重试"})
		return
	}
	current, err := s.Store.Mistake(r.Context(), u.ClassID, u.ID, id)
	if err != nil || current != work {
		status = "stale"
		if err != nil && err != ErrNotFound {
			failure(w, err)
		} else {
			write(w, 409, map[string]string{"error": "错题已更新，请刷新"})
		}
		return
	}
	currentRefs, err := s.Store.ExplanationEvidence(r.Context(), work, hits)
	if err != nil || !reflect.DeepEqual(refs, currentRefs) {
		status = "stale"
		if err != nil && err != ErrConflict {
			failure(w, err)
		} else {
			write(w, 409, map[string]string{"error": "资料已更新，请重新生成讲解"})
		}
		return
	}
	status = "succeeded"
	write(w, 200, result)
}
