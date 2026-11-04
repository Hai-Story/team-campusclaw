// Package mcp implements the course HTTP tools subset, isolated from model orchestration.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	schema "github.com/santhosh-tekuri/jsonschema/v5"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

const Protocol = "2025-03-26"

var toolName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,47}$`)
var serverName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,47}$`)

func ParseRef(ref string) (string, string, error) {
	p := strings.Split(ref, "@")
	if len(p) != 2 || !toolName.MatchString(p[0]) || !serverName.MatchString(p[1]) {
		return "", "", errors.New("工具须为knowledge_search或小写tool@server，工具名不可含连字符")
	}
	return p[0], p[1], nil
}
func normalizeURL(raw string) (string, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return "", errors.New("Base URL须为不含凭据、查询或片段的HTTP(S)地址")
	}
	return strings.TrimRight(u.String(), "/"), nil
}
func compileSchema(raw json.RawMessage) (*schema.Schema, error) {
	if len(raw) > 32<<10 {
		return nil, errors.New("工具输入结构过大")
	}
	var root map[string]any
	if json.Unmarshal(raw, &root) != nil || root["type"] != "object" {
		return nil, errors.New("工具输入结构必须声明object")
	}
	c := schema.NewCompiler()
	c.LoadURL = func(string) (io.ReadCloser, error) { return nil, errors.New("不允许外部schema引用") }
	if e := c.AddResource("urn:campusclaw:input", bytes.NewReader(raw)); e != nil {
		return nil, errors.New("输入结构无效")
	}
	s, e := c.Compile("urn:campusclaw:input")
	if e != nil {
		return nil, errors.New("输入结构无效或包含外部引用")
	}
	return s, nil
}
func ValidateArguments(rawSchema, raw json.RawMessage) error {
	c, e := compileSchema(rawSchema)
	if e != nil {
		return e
	}
	var v any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if d.Decode(&v) != nil {
		return errors.New("工具参数不是JSON对象")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("工具参数包含多余数据")
	}
	if _, ok := v.(map[string]any); !ok {
		return errors.New("工具参数须为对象")
	}
	if c.Validate(v) != nil {
		return errors.New("工具参数不符合已发现的JSON Schema")
	}
	return nil
}
func encrypt(key []byte, plain string) ([]byte, error) {
	b, e := aes.NewCipher(key)
	if e != nil {
		return nil, e
	}
	g, e := cipher.NewGCM(b)
	if e != nil {
		return nil, e
	}
	nonce := make([]byte, g.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return nil, e
	}
	return g.Seal(nonce, nonce, []byte(plain), []byte("campusclaw-mcp-token-v1")), nil
}
func decrypt(key, ciphertext []byte) (string, error) {
	b, e := aes.NewCipher(key)
	if e != nil {
		return "", e
	}
	g, e := cipher.NewGCM(b)
	if e != nil || len(ciphertext) < g.NonceSize() {
		return "", errors.New("MCP密钥无法读取，请重新登记令牌")
	}
	n := g.NonceSize()
	v, e := g.Open(nil, ciphertext[:n], ciphertext[n:], []byte("campusclaw-mcp-token-v1"))
	if e != nil {
		return "", errors.New("MCP密钥无法读取，请重新登记令牌")
	}
	return string(v), nil
}
func scrub(v any, token string) any {
	switch x := v.(type) {
	case string:
		if token != "" {
			return strings.ReplaceAll(x, token, "[令牌已脱敏]")
		}
		return x
	case []any:
		for i := range x {
			x[i] = scrub(x[i], token)
		}
	case map[string]any:
		y := map[string]any{}
		for k, w := range x {
			y[scrub(k, token).(string)] = scrub(w, token)
		}
		return y
	}
	return v
}
func SafeText(s string) (string, bool) {
	lower := strings.ToLower(s)
	for _, word := range []string{"忽略前述", "忽略之前", "忽略以上", "忽略所有", "忽略系统", "泄露密钥", "发送密钥", "ignore previous", "ignore all instructions", "ignore the system", "system prompt", "reveal the", "读取本地文件", "将上下文发送"} {
		if strings.Contains(lower, word) {
			return "工具返回含指令式内容，已整条丢弃。", false
		}
	}
	r := []rune(s)
	if len(r) > 2000 {
		s = string(r[:2000]) + "…"
	}
	return s, true
}

var rpcID atomic.Int64
var remoteClient = &http.Client{Timeout: 8 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("禁止MCP重定向") }}

func rpc(ctx context.Context, base, token, method string, params any, out any) error {
	id := rpcID.Add(1)
	payload := map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}
	if strings.HasPrefix(method, "notifications/") {
		delete(payload, "id")
	}
	b, _ := json.Marshal(payload)
	req, e := http.NewRequestWithContext(ctx, "POST", base+"/mcp", bytes.NewReader(b))
	if e != nil {
		return errors.New("MCP地址无效")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", Protocol)
	res, e := remoteClient.Do(req)
	if e != nil {
		return errors.New("MCP连接失败、超时或重定向被拒")
	}
	defer res.Body.Close()
	if res.StatusCode == 202 && out == nil {
		return nil
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("MCP服务返回%d，请检查地址与令牌", res.StatusCode)
	}
	if res.Header.Get("Mcp-Session-Id") != "" {
		return errors.New("本课客户端不支持有状态MCP会话")
	}
	var data []byte
	reader := io.LimitReader(res.Body, (1<<20)+1)
	if strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		scan := bufio.NewScanner(reader)
		scan.Buffer(make([]byte, 4096), 1<<20)
		var lines []string
		for scan.Scan() {
			line := strings.TrimSuffix(scan.Text(), "\r")
			if strings.HasPrefix(line, "data:") {
				lines = append(lines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			} else if line == "" && len(lines) > 0 {
				candidate := []byte(strings.Join(lines, "\n"))
				lines = nil
				var probe struct {
					ID int64 `json:"id"`
				}
				if json.Unmarshal(candidate, &probe) == nil && probe.ID == id {
					data = candidate
					break
				}
			}
		}
	} else {
		data, e = io.ReadAll(reader)
		if e != nil {
			return errors.New("MCP响应读取失败")
		}
	}
	if len(data) > 1<<20 {
		return errors.New("MCP响应超出大小限制")
	}
	var envelope struct {
		Version string          `json:"jsonrpc"`
		ID      int64           `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	if json.Unmarshal(data, &envelope) != nil || envelope.Version != "2.0" || envelope.ID != id {
		return errors.New("MCP响应报文或id不匹配")
	}
	if len(envelope.Error) > 0 && string(envelope.Error) != "null" {
		return errors.New("MCP工具返回协议错误")
	}
	if len(envelope.Result) == 0 {
		return errors.New("MCP响应缺少result")
	}
	if out != nil {
		var clean any
		if json.Unmarshal(envelope.Result, &clean) != nil {
			return errors.New("MCP响应无效")
		}
		b, _ = json.Marshal(scrub(clean, token))
		if json.Unmarshal(b, out) != nil {
			return errors.New("MCP响应结构错误")
		}
	}
	return nil
}
func fingerprint(t Tool) string {
	b, _ := json.Marshal(struct {
		Name, Description string
		Schema            json.RawMessage
		Annotations       map[string]any
	}{t.Name, t.Description, t.InputSchema, t.Annotations})
	v := sha256.Sum256(b)
	return hex.EncodeToString(v[:])
}
func readOnly(t Tool) bool {
	return t.Annotations["readOnlyHint"] == true && t.Annotations["openWorldHint"] == false && t.Annotations["destructiveHint"] != true
}
