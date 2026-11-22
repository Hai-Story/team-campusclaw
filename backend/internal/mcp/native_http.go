package mcp

import (
	"campusclaw/backend/internal/auth"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
)

func nativeDecode(w http.ResponseWriter, r *http.Request, v any) bool {
	kind, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || kind != "application/json" {
		reply(w, 415, map[string]string{"error": "请使用application/json"})
		return false
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	d.DisallowUnknownFields()
	e = d.Decode(v)
	if e == nil {
		var extra any
		if d.Decode(&extra) != io.EOF {
			e = errors.New("extra data")
		}
	}
	if e != nil {
		var large *http.MaxBytesError
		status := 400
		if errors.As(e, &large) {
			status = 413
		}
		reply(w, status, map[string]string{"error": "请求格式或大小不合法"})
		return false
	}
	return true
}

// @api {"method":"POST","path":"/api/mcp/learning-servers","summary":"幂等添加本班只读学习工具服务","auth":"teacher","status":200,"request":{"type":"object","additionalProperties":false},"response":{"$ref":"#/components/schemas/MCPServer"},"errors":{"400":"正文非法","401":"需要登录","403":"仅教师可操作","404":"身份已变化","409":"保留名称冲突","413":"正文过大","415":"类型错误","503":"学习工具暂时不可用"}}
func (m *Manager) EnableLearning(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.FromContext(r.Context())
	if !ok {
		reply(w, 401, map[string]string{"error": "需要登录"})
		return
	}
	if u.Role != "teacher" {
		reply(w, 403, map[string]string{"error": "仅教师可操作"})
		return
	}
	var body *struct{}
	if !nativeDecode(w, r, &body) {
		return
	}
	if body == nil {
		reply(w, 400, map[string]string{"error": "正文须为对象"})
		return
	}
	v, e := m.registerLearning(r.Context(), u)
	if e != nil {
		problem(w, e)
		return
	}
	reply(w, 200, v)
}
func rpcError(w http.ResponseWriter, id json.RawMessage, code int, msg string) {
	reply(w, 200, map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": msg}})
}

// @api {"method":"POST","path":"/api/learning-tools/mcp","summary":"以当前登录身份访问已批准的本班学习MCP工具","auth":"login","status":200,"request":{"type":"object","additionalProperties":false,"required":["jsonrpc","method"],"properties":{"jsonrpc":{"type":"string","enum":["2.0"]},"id":{},"method":{"type":"string","enum":["initialize","notifications/initialized","tools/list","tools/call"]},"params":{"type":"object"}}},"response":{"type":"object","properties":{"jsonrpc":{"type":"string"},"id":{},"result":{"type":"object"},"error":{"type":"object"}}},"errors":{"400":"请求格式非法","401":"需要登录","403":"角色无权","404":"本班服务未启用或未连接","413":"正文过大","415":"类型错误","429":"每人每分钟最多30次工具尝试","503":"依赖不可用"},"description":"notifications/initialized成功202无正文；业务工具错误以result.isError或JSON-RPC error判定，HTTP200不代表工具成功。"}
func (m *Manager) LearningRPC(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.FromContext(r.Context())
	if !ok {
		reply(w, 401, map[string]string{"error": "需要登录"})
		return
	}
	if u.Role != "teacher" && u.Role != "student" {
		reply(w, 403, map[string]string{"error": "当前角色无权"})
		return
	}
	var in struct {
		Version string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if !nativeDecode(w, r, &in) {
		return
	}
	if in.Version != "2.0" {
		reply(w, 400, map[string]string{"error": "请使用JSON-RPC 2.0"})
		return
	}
	if in.Method != "notifications/initialized" {
		var id any
		d := json.NewDecoder(strings.NewReader(string(in.ID)))
		d.UseNumber()
		if len(in.ID) > 128 || d.Decode(&id) != nil {
			reply(w, 400, map[string]string{"error": "请求标识须为数字或字符串"})
			return
		}
		switch id.(type) {
		case string, json.Number:
		default:
			reply(w, 400, map[string]string{"error": "请求标识须为数字或字符串"})
			return
		}
	}
	server, e := m.learningServer(r.Context(), u.ClassID)
	if e != nil {
		problem(w, e)
		return
	}
	if m.Learning == nil {
		reply(w, 503, map[string]string{"error": "学习工具暂时不可用"})
		return
	}
	result := func(data any) { reply(w, 200, map[string]any{"jsonrpc": "2.0", "id": in.ID, "result": data}) }
	switch in.Method {
	case "initialize":
		result(map[string]any{"protocolVersion": Protocol, "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": "CampusClaw learning", "version": "1"}})
	case "notifications/initialized":
		w.WriteHeader(202)
	case "tools/list":
		tools, e := m.tools(r.Context(), server.ID)
		if e != nil {
			problem(w, e)
			return
		}
		out := []Tool{}
		for _, t := range tools {
			if t.Approved && readOnly(t) && m.currentLearningTool(t) {
				out = append(out, t)
			}
		}
		result(map[string]any{"tools": out})
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		d := json.NewDecoder(strings.NewReader(string(in.Params)))
		d.DisallowUnknownFields()
		if d.Decode(&p) != nil || !toolName.MatchString(p.Name) {
			rpcError(w, in.ID, -32602, "工具参数格式不合法")
			return
		}
		if !m.nativePermit(u.ID, time.Now()) {
			w.Header().Set("Retry-After", "60")
			reply(w, 429, map[string]string{"error": "请求过多，请60秒后重试"})
			return
		}
		tools, e := m.tools(r.Context(), server.ID)
		if e != nil {
			problem(w, e)
			return
		}
		candidate := Candidate{ServerID: server.ID, ServerName: server.Name, Ref: p.Name + "@" + server.Name, Tool: Tool{Name: p.Name}}
		for _, t := range tools {
			if t.Name == p.Name {
				candidate.Tool = t
				break
			}
		}
		call, e := m.Execute(r.Context(), u, 0, 0, candidate, 1, p.Arguments)
		if e != nil {
			problem(w, e)
			return
		}
		result(map[string]any{"content": []map[string]string{{"type": "text", "text": call.Text}}, "isError": call.Status != "ok"})
	default:
		rpcError(w, in.ID, -32601, "不支持的MCP方法")
	}
}
