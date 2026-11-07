package tutor

import (
	"campusclaw/backend/internal/auth"
	"net/http"
	"strconv"
)

// @api {"method":"GET","path":"/api/conversations/{id}/messages/{message}/trace","summary":"读取本人消息已选择记录的LLM实际请求与响应","auth":"student","status":200,"request":null,"response":{"$ref":"#/components/schemas/LLMTrace"},"errors":{"401":"未登录","403":"仅学生本人可查看","404":"消息不存在或未记录调用详情","503":"服务暂时不可用"},"description":"默认不记录。capture_trace=true后包含工具规划和最终SSE的请求体、响应、状态、截断标记；不含鉴权头。每轮最多8次，每次响应128KiB。"}
func (s *Service) MessageTrace(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromContext(r.Context())
	c, err := s.conversation(r.Context(), pathID(r), u)
	if err != nil {
		resourceError(w, err)
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("message"), 10, 64)
	var data string
	err = s.DB.QueryRowContext(r.Context(), `SELECT llm_trace_json FROM messages WHERE id=? AND conversation_id=? AND role='assistant' AND llm_trace_json IS NOT NULL`, id, c.ID).Scan(&data)
	if err != nil {
		resourceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(data))
}
