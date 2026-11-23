package curriculum

import (
	"campusclaw/backend/internal/auth"
	"campusclaw/backend/internal/grading"
	"campusclaw/backend/internal/tutor"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// @api {"method":"POST","path":"/api/walkthroughs","summary":"学生创建本人单题讲题过程","auth":"student","status":201,"request":{"$ref":"#/components/schemas/WalkCreate"},"response":{"$ref":"#/components/schemas/Walkthrough"},"errors":{"400":"输入非法","401":"需要登录","403":"仅学生","404":"作业不存在","409":"创建键冲突","413":"请求过大","415":"类型非法","503":"服务暂不可用"}}
// @api {"method":"GET","path":"/api/walkthroughs","summary":"按学科分页读取本人讲题历史","auth":"student","status":200,"response":{"type":"object","properties":{"items":{"type":"array","items":{"$ref":"#/components/schemas/Walkthrough"}},"page":{"type":"integer"},"page_size":{"type":"integer"}}},"errors":{"400":"筛选非法","401":"需要登录","403":"仅学生","503":"服务暂不可用"}}
// @api {"method":"GET","path":"/api/walkthroughs/{id}","summary":"恢复本人讲题阶段及历史","auth":"student","status":200,"response":{"$ref":"#/components/schemas/Walkthrough"},"errors":{"400":"标识非法","401":"需要登录","403":"仅学生","404":"资源不存在","503":"服务暂不可用"}}
// @api {"method":"POST","path":"/api/walkthroughs/{id}/turns","summary":"SSE讲题讨论、推进及完成","auth":"student","status":200,"request":{"$ref":"#/components/schemas/WalkInput"},"response":{"type":"string"},"stream":true,"description":"meta确认在途；校验并保存后delta.text和done；error表示失败，刷新GET恢复权威状态。已提交结果在断流后仍保留。10秒注释心跳，模型预算121秒、租约130秒。","errors":{"400":"输入非法","401":"需要登录","403":"仅学生","404":"资源不存在","409":"版本、阶段或在途冲突","413":"请求过大","415":"类型非法","503":"依赖暂不可用"}}
func mountWalks(mux *http.ServeMux, s *Service, protect func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("POST /api/walkthroughs", protect(s.CreateWalkHTTP))
	mux.HandleFunc("GET /api/walkthroughs", protect(s.ListWalksHTTP))
	mux.HandleFunc("GET /api/walkthroughs/{id}", protect(s.GetWalkHTTP))
	mux.HandleFunc("POST /api/walkthroughs/{id}/turns", protect(s.WalkTurnHTTP))
}
func (s *Service) CreateWalkHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "student")
	if !ok {
		return
	}
	var in WalkCreate
	if !decode(w, r, &in) {
		return
	}
	if e := in.Validate(); e != nil {
		failure(w, e)
		return
	}
	v, e := s.Store.CreateWalk(r.Context(), u, in)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 201, v)
}
func (s *Service) ListWalksHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "student")
	if !ok {
		return
	}
	f, e := pointFilter(r)
	if e != nil || r.URL.Query().Has("favorites") {
		failure(w, ErrInvalid)
		return
	}
	v, e := s.Store.Walks(r.Context(), u, f)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, map[string]any{"items": v, "page": f.Page, "page_size": f.Size})
}
func (s *Service) GetWalkHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "student")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	v, e := s.Store.Walk(r.Context(), u, id)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}
func sseFrame(w http.ResponseWriter, event string, value any) error {
	b, e := json.Marshal(value)
	if e != nil {
		return e
	}
	if _, e = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b); e != nil {
		return e
	}
	return http.NewResponseController(w).Flush()
}

type walkGeneration struct {
	output WalkOutput
	usage  *ModelUsage
	err    error
}

func (s *Service) walkOutput(ctx context.Context, u auth.User, run WalkRun, in WalkInput) walkGeneration {
	v, e := s.Store.Walk(ctx, u, run.Walk.ID)
	if e != nil {
		return walkGeneration{err: e}
	}
	history := []map[string]string{}
	for _, t := range v.Turns {
		if t.Status == "succeeded" {
			history = append(history, map[string]string{"stage": t.StageAfter, "student": t.Content, "guidance": t.Response})
		}
	}
	if len(history) > 6 {
		history = history[len(history)-6:]
	}
	messages, _ := tutor.BuildMessages(tutor.Agent{}, &tutor.Skill{Name: "解题引导", Description: "先了解学生步骤，指出下一步检查方法。"}, nil, nil)
	skill, e := teachingSkills.ReadFile("skills/walkthrough/SKILL.md")
	if e != nil || s.Generator == nil {
		return walkGeneration{err: ErrModel}
	}
	text, usage, e := s.Generator.Generate(ctx, "walk-"+in.ClientKey, messages[0].Content+"\n"+string(skill), map[string]any{"subject": v.Subject, "question": v.Question, "stage": run.Turn.StageAfter, "action": in.Action, "student": in.Content, "history": history, "previous_summary": v.Summary})
	result := walkGeneration{usage: usage, err: e}
	if e != nil {
		return result
	}
	if e = strictJSON(text, &result.output); e == nil {
		e = result.output.Validate()
	}
	if e == nil && grading.RevealsAnswer(result.output.Response+result.output.Summary+strings.Join(result.output.Improvements, ""), v.Reference) {
		e = ErrModel
	}
	result.err = e
	return result
}
func (s *Service) WalkTurnHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "student")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in WalkInput
	if !decode(w, r, &in) {
		return
	}
	if e := in.Validate(); e != nil {
		failure(w, e)
		return
	}
	run, e := s.Store.BeginWalkTurn(r.Context(), u, id, in)
	if e != nil {
		failure(w, e)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	if run.Replayed {
		if sseFrame(w, "delta", map[string]any{"text": run.Turn.Response}) != nil {
			return
		}
		_ = sseFrame(w, "done", map[string]any{"walkthrough_id": id, "turn_id": run.Turn.ID, "replayed": true})
		return
	}
	success := false
	start := time.Now()
	status := "failed"
	var usage *ModelUsage
	defer func() {
		if !success {
			if r.Context().Err() != nil {
				status = "cancelled"
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = s.Store.FailWalkTurn(ctx, run, status)
		}
		b, _ := json.Marshal(map[string]any{"time": time.Now().UTC().Format(time.RFC3339Nano), "level": "info", "module": "curriculum", "action": "walk.generate", "actor_id": u.ID, "skill_id": WalkVersion, "model": s.ModelName, "duration_ms": time.Since(start).Milliseconds(), "hit_count": 0, "usage": usage, "status": status})
		curriculumLog.Print(string(b))
	}()
	if sseFrame(w, "meta", map[string]any{"skill": map[string]string{"name": "解题引导"}, "stage": run.Turn.StageAfter, "status": "pending"}) != nil {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 121*time.Second)
	defer cancel()
	result := make(chan walkGeneration, 1)
	go func() { result <- s.walkOutput(ctx, u, run, in) }()
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			if r.Context().Err() == nil {
				_ = sseFrame(w, "error", map[string]string{"error": "生成超时，请重试"})
			}
			return
		case <-tick.C:
			if _, e = fmt.Fprint(w, ": processing\n\n"); e != nil {
				return
			}
			if http.NewResponseController(w).Flush() != nil {
				return
			}
		case generated := <-result:
			usage = generated.usage
			if generated.err != nil || ctx.Err() != nil {
				_ = sseFrame(w, "error", map[string]string{"error": "讲题生成失败，请重试"})
				return
			}
			if e = s.Store.FinishWalkTurn(ctx, u, run, generated.output); e != nil {
				_ = sseFrame(w, "error", map[string]string{"error": "过程保存失败，请刷新后重试"})
				return
			}
			success = true
			status = "succeeded"
			if sseFrame(w, "delta", map[string]string{"text": generated.output.Response}) != nil {
				return
			}
			_ = sseFrame(w, "done", map[string]any{"walkthrough_id": id, "turn_id": run.Turn.ID, "stage": run.Turn.StageAfter, "version": run.Walk.Version + 1})
			return
		}
	}
}
