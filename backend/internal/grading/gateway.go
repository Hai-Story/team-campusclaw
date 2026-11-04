package grading

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

//go:embed prompts/feedback-v1.txt
var promptFiles embed.FS

func Feedback(ctx context.Context, c Settings, work Work) (string, Usage, *GradeError) {
	prompt, _ := promptFiles.ReadFile("prompts/feedback-v1.txt")
	input, _ := json.Marshal(map[string]any{"question": work.Question, "answer": work.Answer, "expected_answer": work.Expected, "score": Score(work.Answer, work.Expected, work.MaxScore), "max_score": work.MaxScore})
	body, _ := json.Marshal(map[string]any{"model": c.Model, "temperature": 0, "max_tokens": 512, "messages": []map[string]string{{"role": "system", "content": string(prompt)}, {"role": "user", "content": string(input)}}})
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(c.ChatURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", Usage{}, &GradeError{"gateway_configuration", false}
	}
	req.Header.Set("Authorization", "Bearer "+c.ChatKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Task-ID", work.TaskID)
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		return "", Usage{}, &GradeError{"gateway_unavailable", true}
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		if res.StatusCode == 429 || res.StatusCode >= 500 {
			return "", Usage{}, &GradeError{"gateway_transient", true}
		}
		return "", Usage{}, &GradeError{"gateway_rejected", false}
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage Usage `json:"usage"`
	}
	decoder := json.NewDecoder(io.LimitReader(res.Body, 262145))
	if decoder.Decode(&out) != nil || len(out.Choices) != 1 {
		return "", Usage{}, &GradeError{"gateway_invalid_response", false}
	}
	var feedback struct {
		Feedback string `json:"feedback"`
	}
	d := json.NewDecoder(strings.NewReader(out.Choices[0].Message.Content))
	d.DisallowUnknownFields()
	if d.Decode(&feedback) != nil {
		return "", Usage{}, &GradeError{"gateway_invalid_response", false}
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return "", Usage{}, &GradeError{"gateway_invalid_response", false}
	}
	feedback.Feedback = strings.TrimSpace(feedback.Feedback)
	if n := len([]rune(feedback.Feedback)); n < 1 || n > 2000 || out.Usage.PromptTokens < 0 || out.Usage.CompletionTokens < 0 {
		return "", Usage{}, &GradeError{"gateway_invalid_response", false}
	}
	return feedback.Feedback, out.Usage, nil
}
