package observability

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
)

func TestCallUsageAndMissing(t *testing.T) {
	a := New()
	ctx := With(context.Background(), a)
	Bind(ctx, 12, 34, "test-model")
	c, id := Begin(ctx, "test-model", "planning")
	Usage(c, json.RawMessage(`{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}`))
	Usage(c, json.RawMessage(`{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13}`))
	End(c, id, nil)
	c, id = Begin(ctx, "test-model", "answer")
	Usage(c, json.RawMessage(`{"prompt_tokens":-1,"completion_tokens":2,"total_tokens":1}`))
	End(c, id, errors.New("sensitive payload must not be retained"))
	Outcome(ctx, "partial")
	f := a.Fields()
	u := f["usage"].(map[string]any)
	if f["actor_id"] != int64(12) || f["skill_id"] != 34 || f["calls"] != 2 || u["total_tokens"] != int64(13) || u["missing_calls"] != 1 || u["complete"] != false {
		t.Fatal(f)
	}
	raw, _ := json.Marshal(f)
	if string(raw) == "" {
		t.Fatal("missing metadata")
	}
}
func TestCancelledNoSkillAndParallel(t *testing.T) {
	a := New()
	ctx := With(context.Background(), a)
	Bind(ctx, 1, nil, "fixture")
	cancelled, cancel := context.WithCancel(ctx)
	c, id := Begin(cancelled, "fixture", "stream")
	cancel()
	End(c, id, c.Err())
	if a.Fields()["call_states"].(map[string]int)["cancelled"] != 1 {
		t.Fatal(a.Fields())
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, id := Begin(ctx, "fixture", "planning")
			Usage(c, json.RawMessage(`{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}`))
			End(c, id, nil)
		}()
	}
	wg.Wait()
	u := a.Fields()["usage"].(map[string]any)
	if u["total_tokens"] != int64(60) || u["reported_calls"] != 20 || u["missing_calls"] != 1 {
		t.Fatal(u)
	}
	if New().Fields() != nil {
		t.Fatal("unbound requests must not create teaching audits")
	}
}
