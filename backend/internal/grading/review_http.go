package grading

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"
)

// @api {"method":"GET","path":"/api/assignments/{id}/submissions","summary":"分页查看本班作业的学生作答与初判、终评","auth":"teacher","status":200,"response":{"$ref":"#/components/schemas/SubmissionPage"},"errors":{"400":"分页非法","401":"需要登录","403":"仅教师可查看提交列表","404":"资源不存在","503":"服务暂时不可用"},"parameters":[{"name":"page","in":"query","schema":{"type":"integer","minimum":1,"default":1}},{"name":"page_size","in":"query","schema":{"type":"integer","minimum":1,"default":20},"description":"超过100按100返回"}]}
func (s *Service) ListSubmissions(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := s.Store.Assignment(r.Context(), u.ClassID, id); err != nil {
		failure(w, err)
		return
	}
	page, size, err := pageArgs(r)
	if err != nil {
		write(w, 400, map[string]string{"error": "分页参数非法"})
		return
	}
	items, err := s.Store.Views(r.Context(), u.ClassID, 0, id, "", false, false, page, size)
	if err != nil {
		failure(w, err)
		return
	}
	write(w, 200, map[string]any{"items": items, "page": page, "page_size": size})
}

// @api {"method":"GET","path":"/api/assignments/{id}/my-submission","summary":"读取本人该作业的最新作答，无提交返回null","auth":"student","status":200,"response":{"allOf":[{"$ref":"#/components/schemas/SubmissionView"}],"nullable":true},"errors":{"400":"标识非法","401":"需要登录","403":"仅学生可用","404":"资源不存在","503":"服务暂时不可用"}}
func (s *Service) MySubmission(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "student")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := s.Store.Assignment(r.Context(), u.ClassID, id); err != nil {
		failure(w, err)
		return
	}
	items, err := s.Store.Views(r.Context(), u.ClassID, u.ID, id, "", false, true, 1, 1)
	if err != nil {
		failure(w, err)
		return
	}
	if len(items) == 0 {
		write(w, 200, nil)
	} else {
		write(w, 200, items[0])
	}
}

// @api {"method":"GET","path":"/api/mistakes","summary":"分页读取本人指定学科的有效错题","auth":"student","status":200,"response":{"$ref":"#/components/schemas/SubmissionPage"},"errors":{"400":"学科或分页非法","401":"需要登录","403":"仅学生可用","503":"服务暂时不可用"},"parameters":[{"name":"page","in":"query","schema":{"type":"integer","minimum":1,"default":1}},{"name":"page_size","in":"query","schema":{"type":"integer","minimum":1,"default":20},"description":"超过100按100返回"},{"name":"subject","in":"query","schema":{"type":"string","enum":["chinese","math","english","science","morality"],"default":"math"}}]}
func (s *Service) Mistakes(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "student")
	if !ok {
		return
	}
	page, size, err := pageArgs(r)
	if err != nil {
		write(w, 400, map[string]string{"error": "学科或分页参数非法"})
		return
	}
	subject := r.URL.Query().Get("subject")
	if subject == "" {
		subject = "math"
	}
	items, err := s.Store.Views(r.Context(), u.ClassID, u.ID, 0, subject, true, false, page, size)
	if err != nil {
		failure(w, err)
		return
	}
	write(w, 200, map[string]any{"items": items, "page": page, "page_size": size})
}

// @api {"method":"PUT","path":"/api/submissions/{id}/review","summary":"教师确认本班作答的分数与对错并追加评审历史","auth":"teacher","status":200,"request":{"type":"object","additionalProperties":false,"required":["score","correct","expected_version"],"properties":{"score":{"type":"integer","minimum":0,"maximum":100},"correct":{"type":"boolean"},"feedback":{"type":"string","maxLength":2000},"expected_version":{"type":"integer","minimum":0}}},"response":{"$ref":"#/components/schemas/FinalReview"},"errors":{"400":"分数或终评内容非法","401":"需要登录","403":"仅教师可终评","404":"资源不存在","409":"终评已更新，请刷新后重试","413":"请求过大","415":"请使用application/json","503":"服务暂时不可用"}}
func (s *Service) Review(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var input ReviewInput
	if !decode(w, r, &input) {
		return
	}
	if !input.Validate() {
		write(w, 400, map[string]string{"error": "请填写合法分数、对错及终评版本"})
		return
	}
	start := time.Now()
	result, err := s.Store.Review(r.Context(), u.ClassID, u.ID, id, input)
	if errors.Is(err, ErrInvalid) {
		write(w, 400, map[string]string{"error": "分数须在0至作业满分之间"})
		return
	}
	if errors.Is(err, ErrConflict) {
		write(w, 409, map[string]string{"error": "终评已更新，请刷新后重试"})
		return
	}
	if err != nil {
		failure(w, err)
		return
	}
	entry, _ := json.Marshal(map[string]any{"ts": time.Now().UTC().Format(time.RFC3339Nano), "level": "info", "module": "grading", "event": "teacher_review", "actor_id": u.ID, "submission_id": id, "version": result.Version, "status": "succeeded", "duration_ms": time.Since(start).Milliseconds()})
	os.Stdout.Write(append(entry, '\n'))
	write(w, 200, result)
}
