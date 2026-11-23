package grading

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestActualUsagePresence(t *testing.T) {
	for _, v := range []struct {
		usage    string
		reported bool
	}{{"", false}, {`,"usage":null`, false}, {`,"usage":{}`, false}, {`,"usage":{"prompt_tokens":null,"completion_tokens":12}`, false}, {`,"usage":{"prompt_tokens":20}`, false}, {`,"usage":{"prompt_tokens":0,"completion_tokens":0}`, true}, {`,"usage":{"prompt_tokens":20,"completion_tokens":12}`, true}} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"choices":[{"message":{"content":"{}"}}]`+v.usage+`}`)
		}))
		_, u, e := completion(context.Background(), Settings{ChatURL: server.URL, ChatKey: "fixture", Model: "test"}, "test", "fixture", map[string]any{}, 10)
		server.Close()
		if e != nil || u.Reported != v.reported {
			t.Fatal(u, e, v)
		}
	}
}

func TestInvalidVerdictRetainsUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"message":{"content":"invalid-json"}}],"usage":{"prompt_tokens":20,"completion_tokens":12}}`)
	}))
	defer server.Close()
	_, used, err := Feedback(context.Background(), Settings{ChatURL: server.URL, Model: "test", ChatKey: "fixture"}, Work{TaskID: "fixture"})
	if err == nil || !used.Reported || used.PromptTokens != 20 {
		t.Fatal(used, err)
	}
}
