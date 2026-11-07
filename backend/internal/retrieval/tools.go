package retrieval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	Arguments   string          `json:"arguments,omitempty"`
}
type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// DecideTools asks the real model to choose a tool and its arguments. Final prose is streamed separately.
func (c *ChatClient) DecideTools(ctx context.Context, messages []ChatMessage, tools []ToolDefinition) (ChatMessage, error) {
	in := map[string]any{"model": c.Model, "messages": messages, "tools": tools, "tool_choice": "auto", "parallel_tool_calls": false, "max_tokens": 700, "temperature": 0.2, "stream": false}
	b, e := json.Marshal(in)
	if e != nil {
		return ChatMessage{}, errors.New("工具规划请求无效")
	}
	req, e := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewReader(b))
	if e != nil {
		return ChatMessage{}, errors.New("工具规划地址无效")
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	res, e := httpClient.Do(req)
	traceHTTP(ctx, "tool-planning", b, res, e)
	if e != nil {
		return ChatMessage{}, errors.New("工具规划暂不可用，改用已有信息回答")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		io.Copy(io.Discard, io.LimitReader(res.Body, traceLimit+1))
		return ChatMessage{}, fmt.Errorf("工具规划网关返回%d，改用已有信息回答", res.StatusCode)
	}
	data, e := io.ReadAll(io.LimitReader(res.Body, (2<<20)+1))
	if e != nil || len(data) > 2<<20 {
		return ChatMessage{}, errors.New("工具规划响应超限或中断")
	}
	var out struct {
		Choices []struct {
			Message ChatMessage `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(data, &out) != nil || len(out.Choices) != 1 || len(out.Choices[0].Message.ToolCalls) > 16 {
		return ChatMessage{}, errors.New("工具规划响应无效")
	}
	m := out.Choices[0].Message
	m.Role = "assistant"
	if len(m.Content) > 8000 {
		m.Content = ""
	}
	return m, nil
}
