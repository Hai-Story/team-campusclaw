package curriculum

import (
	"bytes"
	"campusclaw/backend/internal/observability"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrModel = errors.New("model response unavailable")

type ModelUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}
type Generator interface {
	Generate(context.Context, string, string, any) (string, *ModelUsage, error)
}
type ChatGenerator struct {
	BaseURL, APIKey, Model  string
	Client                  *http.Client
	AttemptTimeout, Backoff time.Duration
}

// Generate is read-only: retry only transport/429/5xx, never a malformed successful response.
func (g *ChatGenerator) Generate(ctx context.Context, key, system string, input any) (string, *ModelUsage, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return "", nil, err
	}
	body, _ := json.Marshal(map[string]any{"model": g.Model, "temperature": 0, "max_tokens": 4096, "messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": string(data)}}})
	budget := g.AttemptTimeout
	if budget <= 0 {
		budget = 60 * time.Second
	}
	backoff := g.Backoff
	if backoff <= 0 {
		backoff = time.Second
	}
	client := g.Client
	if client == nil {
		client = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	for attempt := 0; attempt < 2; attempt++ {
		if ctx.Err() != nil {
			return "", nil, ctx.Err()
		}
		call, cancel := context.WithTimeout(ctx, budget)
		skillID := "structured-generation"
		for prefix, name := range map[string]string{"lesson-": LessonVersion, "bank-": BankVersion, "graph-": GraphVersion, "diagnosis-": DiagnosisVersion, "walk-": WalkVersion} {
			if strings.HasPrefix(key, prefix) {
				skillID = name
				break
			}
		}
		observability.Ensure(ctx, skillID, g.Model)
		call, callID := observability.Begin(call, g.Model, "structured")
		req, e := http.NewRequestWithContext(call, "POST", strings.TrimRight(g.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
		if e != nil {
			observability.End(call, callID, e)
			cancel()
			return "", nil, ErrModel
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+g.APIKey)
		req.Header.Set("X-Task-ID", key)
		res, e := client.Do(req)
		retry := e != nil
		if e == nil {
			raw, readErr := io.ReadAll(io.LimitReader(res.Body, 262145))
			res.Body.Close()
			retry = res.StatusCode == 429 || res.StatusCode >= 500
			if res.StatusCode == 200 {
				if readErr == nil && len(raw) <= 262144 {
					var usage struct {
						Usage json.RawMessage `json:"usage"`
					}
					if json.Unmarshal(raw, &usage) == nil {
						var u ModelUsage
						if json.Unmarshal(usage.Usage, &u) == nil && u.PromptTokens >= 0 && u.CompletionTokens >= 0 {
							var fields map[string]json.RawMessage
							if json.Unmarshal(usage.Usage, &fields) == nil && fields["prompt_tokens"] != nil && string(fields["prompt_tokens"]) != "null" && fields["completion_tokens"] != nil && string(fields["completion_tokens"]) != "null" {
								v, _ := json.Marshal(map[string]int{"prompt_tokens": u.PromptTokens, "completion_tokens": u.CompletionTokens, "total_tokens": u.PromptTokens + u.CompletionTokens})
								observability.Usage(call, v)
							}
						}
					}
				}
				observability.End(call, callID, readErr)
				cancel()
				if readErr != nil || len(raw) > 262144 {
					return "", nil, ErrModel
				}
				var out struct {
					Choices []struct {
						Message struct {
							Content string `json:"content"`
						} `json:"message"`
					} `json:"choices"`
					Usage *ModelUsage `json:"usage"`
				}
				if json.Unmarshal(raw, &out) != nil || len(out.Choices) != 1 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
					return "", nil, ErrModel
				}
				if out.Usage != nil && (out.Usage.PromptTokens < 0 || out.Usage.CompletionTokens < 0) {
					return "", nil, ErrModel
				}
				observability.Outcome(ctx, "complete")
				return out.Choices[0].Message.Content, out.Usage, nil
			}
		}
		observability.End(call, callID, ErrModel)
		cancel()
		if ctx.Err() != nil {
			return "", nil, ctx.Err()
		}
		if !retry || attempt == 1 {
			return "", nil, ErrModel
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", nil, ctx.Err()
		case <-timer.C:
		}
	}
	return "", nil, fmt.Errorf("generation: %w", ErrModel)
}

func strictJSON(text string, out any) error {
	d := json.NewDecoder(strings.NewReader(text))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		return ErrModel
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return ErrModel
	}
	return nil
}
