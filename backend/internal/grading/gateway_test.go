package grading

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFeedbackContracts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		retry  bool
		valid  bool
	}{
		{"success", 200, `{"choices": [{"message": {"content": "{\"correct\": true, \"feedback\": \"核对步骤\"}"}}]}`, false, true},
		{"false_valid", 200, `{"choices": [{"message": {"content": "{\"correct\": false, \"feedback\": \"请核对符号\"}"}}]}`, false, true},
		{"transient", 503, `{}`, true, false},
		{"rate_limit", 429, `{}`, true, false},
		{"auth", 401, `{}`, false, false},
		{"score_override", 200, `{"choices": [{"message": {"content": "{\"correct\": true, \"feedback\": \"ok\", \"score\": 0}"}}]}`, false, false},
		{"missing_correct", 200, `{"choices": [{"message": {"content": "{\"feedback\": \"ok\"}"}}]}`, false, false},
		{"string_correct", 200, `{"choices": [{"message": {"content": "{\"correct\": \"false\", \"feedback\": \"ok\"}"}}]}`, false, false},
		{"empty", 200, `{"choices": [{"message": {"content": "{\"correct\": true, \"feedback\": \" \"}"}}]}`, false, false},
		{"negative_usage", 200, `{"choices": [{"message": {"content": "{\"correct\": true, \"feedback\": \"ok\"}"}}], "usage": {"prompt_tokens": -1}}`, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); w.Write([]byte(tc.body)) }))
			defer server.Close()
			feedback, _, err := Feedback(context.Background(), Settings{ChatURL: server.URL}, Work{TaskID: "test"})
			if tc.valid {
				if err != nil || feedback.Feedback == "" {
					t.Fatal("valid feedback rejected")
				}
			} else if err == nil || err.Retry != tc.retry {
				t.Fatal("incorrect error classification")
			}
		})
	}
}
func TestFeedbackCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-time.After(200 * time.Millisecond):
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, _, err := Feedback(ctx, Settings{ChatURL: server.URL}, Work{})
	if err == nil || !err.Retry {
		t.Fatal("timeout not retryable")
	}
}
func TestScoreAndBackoff(t *testing.T) {

	for attempt := 1; attempt <= 3; attempt++ {
		for i := 0; i < 10; i++ {
			delay := retryDelay(time.Second, attempt)
			minimum := time.Second * time.Duration(1<<(attempt-1))
			if delay < minimum || delay > minimum+minimum/5 {
				t.Fatal("backoff outside bounds")
			}
		}
	}
}
