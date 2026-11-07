package retrieval

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Stream passes actual gateway deltas through; never splits a completed answer.
func (c *ChatClient) Stream(ctx context.Context, messages []ChatMessage, emit func(string) error) error {
	body, _ := json.Marshal(map[string]any{"model": c.Model, "messages": messages, "temperature": 0.2, "max_tokens": 500, "stream": true})
	req, e := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	res, e := httpClient.Do(req)
	traceHTTP(ctx, "final-stream", body, res, e)
	if e != nil {
		return errors.New("对话网关连接失败")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		io.Copy(io.Discard, io.LimitReader(res.Body, traceLimit+1))
		return fmt.Errorf("对话网关返回 %d", res.StatusCode)
	}
	scan := bufio.NewScanner(io.LimitReader(res.Body, 20<<20))
	scan.Buffer(make([]byte, 4096), 1<<20)
	var data []string
	done := false
	process := func() error {
		if len(data) == 0 {
			return nil
		}
		v := strings.Join(data, "\n")
		data = nil
		if v == "[DONE]" {
			done = true
			return nil
		}
		var frame struct {
			Error   json.RawMessage `json:"error"`
			Usage   json.RawMessage `json:"usage"`
			Choices []struct {
				FinishReason string `json:"finish_reason"`
				Delta        struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(v), &frame) != nil {
			return errors.New("对话流格式错误")
		}
		if len(frame.Error) > 0 && string(frame.Error) != "null" {
			return errors.New("对话流返回错误")
		}
		if len(frame.Choices) > 0 {
			traceDelta(ctx, frame.Choices[0].Delta.Content, frame.Choices[0].FinishReason, frame.Usage)
		} else {
			traceDelta(ctx, "", "", frame.Usage)
		}
		if len(frame.Choices) > 0 && frame.Choices[0].Delta.Content != "" {
			return emit(frame.Choices[0].Delta.Content)
		}
		return nil
	}
	for scan.Scan() {
		line := strings.TrimSuffix(scan.Text(), "\r")
		if line == "" {
			if e = process(); e != nil {
				return e
			}
			if done {
				return nil
			}
		} else if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if e = scan.Err(); e != nil {
		return errors.New("对话流中断")
	}
	if e = process(); e != nil {
		return e
	}
	if !done {
		return errors.New("对话流未正常结束")
	}
	return nil
}
