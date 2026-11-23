// Package observability collects identifiers and numeric model usage, never teaching content.
package observability

import (
	"context"
	"encoding/json"
	"sync"
)

type contextKey struct{}
type activeKey struct{}
type call struct {
	model, phase, status string
	usage                *tokenUsage
}
type tokenUsage struct {
	Prompt     int64 `json:"prompt_tokens"`
	Completion int64 `json:"completion_tokens"`
	Total      int64 `json:"total_tokens"`
}
type Audit struct {
	mu             sync.Mutex
	actor          int64
	skill          any
	model, outcome string
	bound          bool
	calls          []call
}

func New() *Audit { return &Audit{outcome: "failed"} }
func With(ctx context.Context, a *Audit) context.Context {
	return context.WithValue(ctx, contextKey{}, a)
}
func get(ctx context.Context) *Audit { a, _ := ctx.Value(contextKey{}).(*Audit); return a }
func Bind(ctx context.Context, actor int64, skill any, model string) {
	if a := get(ctx); a != nil {
		a.mu.Lock()
		defer a.mu.Unlock()
		a.actor = actor
		a.skill = skill
		a.model = model
		a.bound = true
	}
}
func Outcome(ctx context.Context, status string) {
	if a := get(ctx); a != nil {
		a.mu.Lock()
		defer a.mu.Unlock()
		a.outcome = status
	}
}
func Begin(ctx context.Context, model, phase string) (context.Context, int) {
	a := get(ctx)
	if a == nil {
		return ctx, -1
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	id := len(a.calls)
	a.calls = append(a.calls, call{model: model, phase: phase, status: "pending"})
	return context.WithValue(ctx, activeKey{}, id), id
}
func End(ctx context.Context, id int, err error) {
	a := get(ctx)
	if a == nil || id < 0 {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if id >= len(a.calls) {
		return
	}
	status := "complete"
	if err != nil {
		status = "failed"
		if ctx.Err() != nil {
			status = "cancelled"
		}
	}
	a.calls[id].status = status
}

// Usage frames are cumulative for a call: replace, never add the same call twice.
func Usage(ctx context.Context, raw json.RawMessage) {
	a := get(ctx)
	id, ok := ctx.Value(activeKey{}).(int)
	if a == nil || !ok {
		return
	}
	var v map[string]json.RawMessage
	if json.Unmarshal(raw, &v) != nil {
		return
	}
	var u tokenUsage
	for key, target := range map[string]*int64{"prompt_tokens": &u.Prompt, "completion_tokens": &u.Completion, "total_tokens": &u.Total} {
		r, ok := v[key]
		if !ok || string(r) == "null" || json.Unmarshal(r, target) != nil || *target < 0 {
			return
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if id >= 0 && id < len(a.calls) {
		a.calls[id].usage = &u
	}
}

// Fields contains no error strings, prompts, URLs, headers, tool arguments or model output.
func (a *Audit) Fields() map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.bound {
		return nil
	}
	var u tokenUsage
	reported := 0
	states := map[string]int{"complete": 0, "failed": 0, "cancelled": 0, "pending": 0}
	for _, c := range a.calls {
		states[c.status]++
		if c.usage != nil {
			reported++
			u.Prompt += c.usage.Prompt
			u.Completion += c.usage.Completion
			u.Total += c.usage.Total
		}
	}
	return map[string]any{"actor_id": a.actor, "skill_id": a.skill, "model": a.model, "status": a.outcome, "calls": len(a.calls), "call_states": states, "usage": map[string]any{"prompt_tokens": u.Prompt, "completion_tokens": u.Completion, "total_tokens": u.Total, "reported": reported > 0, "complete": len(a.calls) > 0 && reported == len(a.calls), "reported_calls": reported, "missing_calls": len(a.calls) - reported}}
}

// Identity does not create a generation event for non-model requests.
func Identity(ctx context.Context, actor int64) {
	if a := get(ctx); a != nil {
		a.mu.Lock()
		defer a.mu.Unlock()
		a.actor = actor
	}
}
func Ensure(ctx context.Context, skill, model string) {
	if a := get(ctx); a != nil {
		a.mu.Lock()
		defer a.mu.Unlock()
		if !a.bound {
			a.skill = skill
			a.model = model
			a.bound = true
		}
	}
}
