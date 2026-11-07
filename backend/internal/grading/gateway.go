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

const VerdictVersion = "grading-verdict-v3"

//go:embed prompts/verdict-v3.txt prompts/explanation-v1.txt
var promptFiles embed.FS

func Feedback(ctx context.Context, c Settings, work Work) (Result, Usage, *GradeError) {
	prompt, _ := promptFiles.ReadFile("prompts/verdict-v3.txt")
	text, usage, failure := completion(ctx, c, work.TaskID, string(prompt), map[string]any{"question": work.Question, "answer": work.Answer, "expected_answer": work.Expected, "subject": work.Subject}, 512)
	if failure != nil {
		return Result{}, usage, failure
	}
	var feedback struct {
		Feedback string `json:"feedback"`
		Correct  *bool  `json:"correct"`
	}
	d := json.NewDecoder(strings.NewReader(text))
	d.DisallowUnknownFields()
	if d.Decode(&feedback) != nil || feedback.Correct == nil {
		return Result{}, Usage{}, &GradeError{"gateway_invalid_response", false}
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return Result{}, Usage{}, &GradeError{"gateway_invalid_response", false}
	}
	feedback.Feedback = strings.TrimSpace(feedback.Feedback)
	if n := len([]rune(feedback.Feedback)); n < 1 || n > 2000 {
		return Result{}, Usage{}, &GradeError{"gateway_invalid_response", false}
	}
	return Result{Correct: *feedback.Correct, Feedback: feedback.Feedback}, usage, nil
}

func completion(ctx context.Context, c Settings, correlation, prompt string, data map[string]any, maxTokens int) (string, Usage, *GradeError) {
	input, _ := json.Marshal(data)
	body, _ := json.Marshal(map[string]any{"model": c.Model, "temperature": 0, "max_tokens": maxTokens, "messages": []map[string]string{{"role": "system", "content": prompt}, {"role": "user", "content": string(input)}}})
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(c.ChatURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", Usage{}, &GradeError{"gateway_configuration", false}
	}
	req.Header.Set("Authorization", "Bearer "+c.ChatKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Task-ID", correlation)
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
	if decoder.Decode(&out) != nil || len(out.Choices) != 1 || out.Usage.PromptTokens < 0 || out.Usage.CompletionTokens < 0 {
		return "", Usage{}, &GradeError{"gateway_invalid_response", false}
	}
	return out.Choices[0].Message.Content, out.Usage, nil
}
