package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
)

const protocol = "2025-03-26"

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"inputSchema"`
	Annotations map[string]any `json:"annotations"`
}

func tools() []tool {
	annotations := map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false}
	return []tool{
		{"formula_hint", "学生卡在去括号、分数或面积步骤时，提供公式和检查方法；不计算最终答案。query填写学生当前疑问。", map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string", "minLength": 1, "maxLength": 500}}, "required": []string{"query"}, "additionalProperties": false}, annotations},
		{"unit_convert", "学生需要长度/质量单位换算方法时使用。给出单位进率和运算方向，不输出代入数值后的最终答案。value为原数值，from/to使用m、cm、mm、km、kg、g。", map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"type": "number", "minimum": 0, "maximum": 1e9}, "from": map[string]any{"type": "string", "enum": []string{"m", "cm", "mm", "km", "kg", "g"}}, "to": map[string]any{"type": "string", "enum": []string{"m", "cm", "mm", "km", "kg", "g"}}}, "required": []string{"value", "from", "to"}, "additionalProperties": false}, annotations},
	}
}
func execute(name string, raw json.RawMessage) (string, error) {
	var args map[string]any
	if json.Unmarshal(raw, &args) != nil || args == nil {
		return "", fmt.Errorf("arguments必须为对象")
	}
	switch name {
	case "formula_hint":
		q, ok := args["query"].(string)
		if !ok || len(args) != 1 || strings.TrimSpace(q) == "" || len([]rune(q)) > 500 {
			return "", fmt.Errorf("query须为1到500字，且不得含其他参数")
		}
		if strings.Contains(q, "分数") {
			return "分数运算提示：先找公分母，分子分母同乘相同的数，再处理分子。请自己完成计算并检查是否可约分。", nil
		}
		if strings.Contains(q, "面积") {
			return "面积提示：先统一长度单位，长方形面积=长×宽；若需要换单位，可再使用unit_convert确认进率。请自行代入并计算。", nil
		}
		return "去括号提示：括号前的系数须乘到括号内每一项；负号也要分配。逐项检查符号与漏乘，不直接计算最终得数。", nil
	case "unit_convert":
		value, ok := args["value"].(float64)
		from, fok := args["from"].(string)
		to, tok := args["to"].(string)
		if !ok || !fok || !tok || len(args) != 3 || math.IsNaN(value) || value < 0 || value > 1e9 {
			return "", fmt.Errorf("value须为0到1e9的数值，from/to须为单位字符串")
		}
		units := map[string]float64{"km": 1000, "m": 1, "cm": .01, "mm": .001, "kg": 1000, "g": 1}
		f, fa := units[from]
		t, ta := units[to]
		weight := func(s string) bool { return s == "kg" || s == "g" }
		if !fa || !ta || weight(from) != weight(to) {
			return "", fmt.Errorf("仅支持同类长度或质量单位换算")
		}
		ratio := f / t
		if ratio >= 1 {
			return fmt.Sprintf("单位换算提示：从%s到%s，原数值乘以%g；请自行计算并在结果后写上%s。", from, to, ratio, to), nil
		}
		return fmt.Sprintf("单位换算提示：从%s到%s，原数值除以%g；请自行计算并在结果后写上%s。", from, to, 1/ratio, to), nil
	default:
		return "", fmt.Errorf("未知工具")
	}
}
func handler(token string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		expected := "Bearer " + token
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte(expected)) != 1 {
			w.WriteHeader(401)
			io.WriteString(w, `{"error":"需要有效Bearer令牌"}`)
			return
		}
		if r.Header.Get("Origin") != "" {
			w.WriteHeader(403)
			io.WriteString(w, `{"error":"不接受浏览器跨源访问"}`)
			return
		}
		if r.Method != "POST" {
			w.Header().Set("Allow", "POST")
			w.WriteHeader(405)
			return
		}
		var in struct {
			Version string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		rpcError := func(code int, msg string) {
			id := in.ID
			if len(id) == 0 {
				id = json.RawMessage("null")
			}
			json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": msg}})
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		if dec.Decode(&in) != nil {
			rpcError(-32700, "JSON解析失败")
			return
		}
		var extra any
		if dec.Decode(&extra) != io.EOF {
			rpcError(-32600, "仅接受单个JSON-RPC请求")
			return
		}
		if in.Version != "2.0" || in.Method == "" {
			rpcError(-32600, "无效JSON-RPC请求")
			return
		}
		if len(in.ID) == 0 {
			if in.Method == "notifications/initialized" {
				w.WriteHeader(202)
				return
			}
			rpcError(-32600, "请求必须包含id")
			return
		}
		var id any
		if json.Unmarshal(in.ID, &id) != nil {
			rpcError(-32600, "无效id")
			return
		}
		switch id.(type) {
		case string, float64:
		default:
			rpcError(-32600, "id须为字符串或数字")
			return
		}
		var result any
		switch in.Method {
		case "initialize":
			var p struct {
				Version string `json:"protocolVersion"`
			}
			if json.Unmarshal(in.Params, &p) != nil || p.Version != protocol {
				rpcError(-32602, "支持的protocolVersion为2025-03-26")
				return
			}
			result = map[string]any{"protocolVersion": protocol, "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "campus-tools", "version": "1.0.0"}}
		case "ping":
			result = map[string]any{}
		case "tools/list":
			result = map[string]any{"tools": tools()}
		case "tools/call":
			var p struct {
				Name string          `json:"name"`
				Args json.RawMessage `json:"arguments"`
			}
			if json.Unmarshal(in.Params, &p) != nil || p.Name == "" {
				rpcError(-32602, "缺少工具名称或参数")
				return
			}
			text, err := execute(p.Name, p.Args)
			if err != nil {
				rpcError(-32602, err.Error())
				return
			}
			result = map[string]any{"content": []map[string]string{{"type": "text", "text": text}}, "isError": false}
		default:
			rpcError(-32601, "不支持的方法")
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": in.ID, "result": result})
	})
	return mux
}
func main() {
	token := os.Getenv("MCP_AUTH_TOKEN")
	if len(token) < 24 {
		log.Fatal("MCP_AUTH_TOKEN必填且至少24字符")
	}
	port := os.Getenv("MCP_PORT")
	if port == "" {
		port = "8090"
	}
	log.Printf("教学MCP服务监听:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, handler(token)))
}
