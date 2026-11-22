package curriculum

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func sampleLesson() LessonOutput {
	return LessonOutput{Goals: []string{"理解分数"}, KeyPoints: []string{"保持单位一致"}, Difficulties: []string{"说明分母的含义"}, Activities: []LessonActivity{{Title: "观察与讨论", Minutes: 45, Steps: []string{"比较讲义中的例子"}}}, Resources: []string{"本班讲义"}, CitationIndices: []int{1}}
}
func TestLessonInputAndOutputContract(t *testing.T) {
	// L1-S1/S2: input constraints and actual duration/citation invariants.
	good := LessonInput{Subject: "math", Chapter: " 分数 ", Objectives: "理解分数", Periods: 1, MinutesPerPeriod: 45, MaterialIDs: []int64{1}, ClientKey: "request-1"}
	if good.Validate() != nil || good.Chapter != "分数" {
		t.Fatal(good)
	}
	for _, modify := range []func(*LessonInput){func(p *LessonInput) { p.Subject = "all" }, func(p *LessonInput) { p.Chapter = " " }, func(p *LessonInput) { p.Objectives = "" }, func(p *LessonInput) { p.Periods = 0 }, func(p *LessonInput) { p.Periods = 9 }, func(p *LessonInput) { p.MinutesPerPeriod = 91 }, func(p *LessonInput) { p.MinutesPerPeriod = 9 }, func(p *LessonInput) { p.MaterialIDs = []int64{1, 1} }, func(p *LessonInput) { p.MaterialIDs = nil }, func(p *LessonInput) { p.ClientKey = "bad key" }} {
		p := good
		modify(&p)
		if p.Validate() == nil {
			t.Fatal("invalid lesson input accepted", p)
		}
	}
	hash := good.Hash()
	same := good
	same.ClientKey = "another-key"
	if same.Hash() != hash {
		t.Fatal("request key included in semantic input hash")
	}
	same.Objectives += "不同"
	if same.Hash() == hash {
		t.Fatal("different input shared hash")
	}
	goodOutput := sampleLesson()
	if goodOutput.Validate(45, 1) != nil || !strings.Contains(goodOutput.Body(), "45分钟") {
		t.Fatal(goodOutput)
	}
	for _, modify := range []func(*LessonOutput){func(p *LessonOutput) { p.Goals = nil }, func(p *LessonOutput) {
		p.Activities = []LessonActivity{{Title: "活动", Minutes: 44, Steps: []string{"讨论"}}}
	}, func(p *LessonOutput) { p.CitationIndices = []int{2} }, func(p *LessonOutput) { p.CitationIndices = []int{0} }, func(p *LessonOutput) { p.CitationIndices = []int{1, 1} }, func(p *LessonOutput) { p.Resources = []string{" "} }} {
		p := sampleLesson()
		modify(&p)
		if p.Validate(45, 1) == nil {
			t.Fatal("invalid generated lesson accepted", p)
		}
	}
	for _, raw := range []string{`{"goals":[],"score":100}`, `{"activities":[{"minutes":1.5}]}`, `{} {}`, "```json\n{}\n```"} {
		var p LessonOutput
		if strictJSON(raw, &p) == nil && p.Validate(45, 1) == nil {
			t.Fatal("unsafe output accepted", raw)
		}
	}
}
func TestGenerationRetryAndInvalidResponses(t *testing.T) {
	for _, tt := range []struct {
		name    string
		status  int
		raw     string
		calls   int
		success bool
	}{{"transient", 503, `{"choices":[{"message":{"content":"{}"}}],"usage":{"prompt_tokens":2,"completion_tokens":3}}`, 2, true}, {"unauthorized", 401, `{}`, 1, false}, {"bad_json", 200, `bad`, 1, false}, {"empty_choices", 200, `{"choices":[]}`, 1, false}, {"bad_usage", 200, `{"choices":[{"message":{"content":"{}"}}],"usage":{"prompt_tokens":-1}}`, 1, false}} {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := int(calls.Add(1))
				if r.Header.Get("Authorization") != "Bearer test-only-key" || r.Header.Get("X-Task-ID") != "lesson-test" {
					t.Error("missing gateway credentials/correlation")
				}
				if n == 1 && tt.status != 200 {
					w.WriteHeader(tt.status)
					return
				}
				fmt.Fprint(w, tt.raw)
			}))
			defer srv.Close()
			g := ChatGenerator{BaseURL: srv.URL, APIKey: "test-only-key", Model: "test-model", Backoff: time.Millisecond}
			out, usage, err := g.Generate(context.Background(), "lesson-test", "teacher skill", map[string]string{"chapter": "fractions"})
			if (err == nil) != tt.success || int(calls.Load()) != tt.calls {
				t.Fatal(out, usage, err, calls.Load())
			}
			if tt.success && (usage == nil || usage.PromptTokens != 2 || usage.CompletionTokens != 3) {
				t.Fatal(usage)
			}
		})
	}
}
func TestGenerationCancelDoesNotRetry(t *testing.T) {
	// L3-S1: cancel the real HTTP request, rather than faking a cancellation result.
	seen := make(chan struct{}, 1)
	cancelSeen := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		calls.Add(1)
		seen <- struct{}{}
		select {
		case <-r.Context().Done():
			close(cancelSeen)
		case <-release:
		}
	}))
	defer srv.Close()
	defer close(release)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	g := ChatGenerator{BaseURL: srv.URL, Backoff: time.Millisecond}
	go func() { _, _, err := g.Generate(ctx, "cancel-test", "skill", struct{}{}); done <- err }()
	<-seen
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancel succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not stop HTTP")
	}
	if calls.Load() != 1 {
		t.Fatal("cancel retried", calls.Load())
	}
	select {
	case <-cancelSeen:
	case <-time.After(time.Second):
		t.Fatal("gateway did not observe cancellation")
	}
}
func TestForgedCitationInBody(t *testing.T) {
	p := sampleLesson()
	p.Goals = []string{"参考[999]"}
	if p.Validate(45, 1) == nil {
		t.Fatal("forged citation survived validation")
	}
}

func TestLessonOutputJSON(t *testing.T) {
	b, err := json.Marshal(sampleLesson())
	if err != nil {
		t.Fatal(err)
	}
	var p LessonOutput
	if strictJSON(string(b), &p) != nil || p.Validate(45, 1) != nil {
		t.Fatal("legitimate structure rejected")
	}
}
