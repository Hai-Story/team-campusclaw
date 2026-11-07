package tutor

import (
	"campusclaw/backend/internal/auth"
	"campusclaw/backend/internal/mcp"
	"campusclaw/backend/internal/retrieval"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

func frame(w http.ResponseWriter, event string, payload any) error {
	data, e := json.Marshal(payload)
	if e != nil {
		return e
	}
	if _, e = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data); e != nil {
		return e
	}
	return http.NewResponseController(w).Flush()
}

// SendMessage validates before starting the stream. Locks are per conversation, never global.
// @api {"method":"POST","path":"/api/conversations/{id}/messages","summary":"在锁定智能体的会话中流式提问","auth":"student","status":200,"request":{"type":"object","properties":{"content":{"type":"string","minLength":2,"maxLength":500},"capture_trace":{"type":"boolean","default":false,"description":"记录本轮实际LLM请求体和原始响应，仅本人查看；无鉴权头"}},"required":["content"]},"response":{"type":"string"},"errors":{"400":"参数非法或本班名称重复","401":"未登录","403":"仅学生可使用智能体会话","404":"资源不存在（他班与不存在同形）","503":"服务暂时不可用","409":"该会话正在生成，或智能体已删除使会话只读"},"stream":true,"description":"真实SSE：首个meta含skill/citations；工具阶段每次meta.tool_calls增量追加真实状态，全部在首个delta前。模型原生多轮tools/call，默认上限3轮、每问6次、会话30次；失败/超限使用已有资料继续回答。delta.text为新增片段，落库后done.message_id；失败error、已生成消息含工具记录保存partial。不同会话并行，同会话409。客户端agent_id/history/class_id被忽略。 智能体删除后的历史仍可读取，但提问409且不新增消息。 capture_trace默认false；开启后消息has_trace=true，可经trace接口读取；响应128KiB/次、最多8次，超限明确标记。 首个meta含user_created_at/assistant_created_at（RFC3339 UTC），助手开始生成时间与保存记录一致，done/error在保存后含created_at；时间只作界面元数据，不传入LLM。","example":"event: meta\ndata: {\"skill\":null,\"citations\":[],\"user_created_at\":\"2026-10-23T00:05:00Z\",\"assistant_created_at\":\"2026-10-23T00:05:01Z\"}\n\nevent: meta\ndata: {\"tool_calls\":[{\"ref\":\"formula_hint@campus-tools\",\"status\":\"ok\",\"text\":\"检查乘法分配律\",\"round\":1}]}\n\nevent: delta\ndata: {\"text\":\"先看这一步\"}\n\nevent: done\ndata: {\"message_id\":1,\"created_at\":\"2026-10-23T00:05:01Z\"}\n\n"}
func (s *Service) SendMessage(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromContext(r.Context())
	c, e := s.conversation(r.Context(), pathID(r), u)
	if e != nil {
		resourceError(w, e)
		return
	}
	var in struct {
		Content      string `json:"content"`
		CaptureTrace bool   `json:"capture_trace"`
	}
	if !body(w, r, &in) {
		return
	}
	in.Content = strings.TrimSpace(in.Content)
	n := len([]rune(in.Content))
	if n < 2 || n > 500 {
		fail(w, 400, "问题须为2到500字")
		return
	}
	if _, loaded := s.busy.LoadOrStore(c.ID, true); loaded {
		fail(w, 409, "该会话正在生成，请等待或停止后再发送")
		return
	}
	defer s.busy.Delete(c.ID)
	// Re-read after claiming the conversation: a concurrent delete may have won.
	c, e = s.conversation(r.Context(), c.ID, u)
	if e != nil {
		resourceError(w, e)
		return
	}
	gate := s.agentGate(c.AgentID)
	gate.RLock()
	defer gate.RUnlock()
	// Deletion may have completed while acquiring the agent gate.
	c, e = s.conversation(r.Context(), c.ID, u)
	if e != nil {
		resourceError(w, e)
		return
	}
	if c.AgentDeleted {
		fail(w, 409, "该智能体已删除，此对话仅可查看或删除")
		return
	}
	a, e := s.agent(r.Context(), c.AgentID, u.ClassID, false)
	if e != nil {
		resourceError(w, e)
		return
	}
	skills, e := s.skills(r.Context(), u.ClassID, a.ID)
	if e != nil {
		dbFail(w)
		return
	}
	sk := Route(skills, in.Content)
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		dbFail(w)
		return
	}
	var count int
	var requestID int64
	e = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM messages WHERE conversation_id=?`, c.ID).Scan(&count)
	if e == nil {
		res, err := tx.ExecContext(r.Context(), `INSERT INTO messages(conversation_id,role,content,citations_json) VALUES(?,'user',?,'[]')`, c.ID, in.Content)
		e = err
		if e == nil {
			requestID, e = res.LastInsertId()
		}
	}
	if e == nil {
		if count == 0 {
			_, e = tx.ExecContext(r.Context(), `UPDATE conversations SET title=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`, short(in.Content, 24), c.ID)
		} else {
			_, e = tx.ExecContext(r.Context(), `UPDATE conversations SET updated_at=CURRENT_TIMESTAMP WHERE id=?`, c.ID)
		}
	}
	if e != nil {
		tx.Rollback()
		dbFail(w)
		return
	}
	if e = tx.Commit(); e != nil {
		dbFail(w)
		return
	}
	materialIDs, e := s.evidenceMaterials(r.Context(), a, in.Content, u.ClassID)
	if e != nil {
		dbFail(w)
		return
	}
	hits, e := s.Searcher.SearchBound(r.Context(), u.ClassID, in.Content, materialIDs, 4)
	if e != nil {
		dbFail(w)
		return
	}
	history, e := s.messages(r.Context(), c.ID, true)
	if e != nil {
		dbFail(w)
		return
	}
	messages, cites := BuildMessages(a, sk, history, hits)
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	assistantStarted := time.Now().Unix()
	if e = frame(w, "meta", map[string]any{"status": "ok", "skill": sk, "citations": cites, "agent_id": a.ID, "user_created_at": history[len(history)-1].Created, "assistant_created_at": timestamp(assistantStarted)}); e != nil {
		return
	}
	filter := CitationFilter{max: len(cites)}
	var answer strings.Builder
	emit := func(text string) error {
		if text == "" {
			return nil
		}
		answer.WriteString(text)
		return frame(w, "delta", map[string]string{"text": text})
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	var trace *retrieval.Trace
	if in.CaptureTrace {
		trace = retrieval.NewTrace(s.Chat.APIKey, os.Getenv("EMBEDDING_API_KEY"), os.Getenv("MCP_AUTH_TOKEN"), os.Getenv("SESSION_SECRET"), os.Getenv("MCP_ENCRYPTION_KEY"), os.Getenv("MYSQL_PASSWORD"))
		ctx = retrieval.WithTrace(ctx, trace)
	}
	var toolCalls []mcp.Call
	messages, toolCalls, streamErr := s.runTools(ctx, u, c.ID, requestID, sk, messages, func(calls []mcp.Call) error { return frame(w, "meta", map[string]any{"tool_calls": calls}) })
	if streamErr == nil {
		streamErr = s.Chat.Stream(ctx, messages, func(text string) error { return emit(filter.Feed(text, false)) })
	}
	if streamErr == nil {
		streamErr = emit(filter.Feed("", true))
	}
	status := "complete"
	if streamErr == nil && answer.Len() == 0 {
		streamErr = fmt.Errorf("回答未生成")
	}
	if streamErr != nil {
		status = "partial"
	}
	// Request cancellation cannot prevent recording text already sent to the student.
	saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer saveCancel()
	if answer.Len() == 0 && len(toolCalls) == 0 && (trace == nil || !trace.HasCalls()) {
		_ = frame(w, "error", map[string]string{"error": "回答未生成，请稍后重试"})
		return
	}
	visible := projectReferences(a, Message{Content: answer.String(), Citations: cites}, in.Content)
	cites = visible.Citations
	if streamErr == nil {
		_ = frame(w, "meta", map[string]any{"citations": cites})
	}
	cj, _ := json.Marshal(cites)
	tj, _ := json.Marshal(toolCalls)
	skillName := ""
	if sk != nil {
		skillName = sk.Name
	}
	var traceJSON any
	if trace != nil && trace.HasCalls() {
		traceJSON = string(trace.JSON())
	}
	res, e := s.DB.ExecContext(saveCtx, `INSERT INTO messages(conversation_id,role,content,citations_json,skill_name,status,tool_calls_json,llm_trace_json,created_at) VALUES(?,'assistant',?,?,?,?,?,?,FROM_UNIXTIME(?))`, c.ID, answer.String(), string(cj), skillName, status, string(tj), traceJSON, assistantStarted)
	if e != nil {
		_ = frame(w, "error", map[string]string{"error": "回答保存失败"})
		return
	}
	id, e := res.LastInsertId()
	if e != nil {
		_ = frame(w, "error", map[string]string{"error": "回答保存失败"})
		return
	}
	if streamErr != nil {
		_ = frame(w, "error", map[string]any{"error": "回答中断，已保存生成的部分内容", "message_id": id, "created_at": timestamp(assistantStarted)})
		return
	}
	_ = frame(w, "done", map[string]any{"message_id": id, "created_at": timestamp(assistantStarted)})
}
