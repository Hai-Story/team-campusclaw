package grading

import (
	"campusclaw/backend/internal/retrieval"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExplanationContracts(t *testing.T) {
	// A13: strict content, no reference-answer transmission, and obvious answer leakage.
	for _, tc := range []struct {
		name, content string
		valid         bool
	}{
		{"hint", `{"explanation":"回想相反数的定义，检查符号与绝对值是否满足要求。","citation_indices":[1]}`, true},
		{"answer", `{"explanation":"相反数为3。","citation_indices":[1]}`, false},
		{"decimal_answer", `{"explanation":"结果为+3.0。","citation_indices":[1]}`, false},
		{"forged_ref", `{"explanation":"回顾定义。","citation_indices":[99]}`, false},
		{"duplicate_ref", `{"explanation":"回顾定义。","citation_indices":[1,1]}`, false},
		{"no_ref", `{"explanation":"回顾定义。","citation_indices":[]}`, false},
		{"extra", `{"explanation":"回想定义。","score":0}`, false},
		{"empty", `{"explanation":" "}`, false},
		{"two_objects", `{"explanation":"回想定义。"}{}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					Messages []struct {
						Content string `json:"content"`
					} `json:"messages"`
				}
				if json.NewDecoder(r.Body).Decode(&request) != nil || len(request.Messages) != 2 {
					t.Error("invalid model request")
				}
				if strings.Contains(request.Messages[1].Content, "expected_answer") {
					t.Error("reference sent to explanation model")
				}
				json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": tc.content}}}})
			}))
			defer server.Close()
			text, _, err := ExplainEvidence(context.Background(), Settings{ChatURL: server.URL}, Work{Expected: "3", Subject: "math", Question: "相反数", Answer: "-3"}, []ExplanationCitation{{1, retrieval.Hit{MaterialID: 1, ChunkID: 1, MaterialTitle: "数学", Excerpt: "相反数的概念。"}}})
			if (err == nil) != tc.valid || (tc.valid && text.Explanation == "") {
				t.Fatalf("unexpected acceptance: %v", err)
			}
		})
	}
}

func TestReferenceAnswerGuard(t *testing.T) {
	for _, tc := range []struct {
		text, ref string
		blocked   bool
	}{
		{"检查负数-3的符号。", "3", false}, {"取相反数得到3。", "3", true},
		{"结果是-3。", "-3", true}, {"选择B。", "B", true}, {"请核对语境和词性。", "red", false},
		{"应为红色。", "红色", true}, {"最终答案是其他。", "参考", true},
	} {
		if revealsAnswer(tc.text, tc.ref) != tc.blocked {
			t.Errorf("incorrect answer guard: %s", tc.text)
		}
	}
}

func TestExplanationSafeSources(t *testing.T) {
	// E1/E2: canonical metadata retained, source answer content hidden.
	refs := []ExplanationCitation{{1, retrieval.Hit{MaterialID: 7, ChunkID: 8, MaterialTitle: "答案是3", Excerpt: "正确答案是3"}}}
	safe := safeExplanationSources(refs, "3")
	if revealsAnswer(safe[0].MaterialTitle, "3") || revealsAnswer(safe[0].Excerpt, "3") || safe[0].MaterialID != 7 || refs[0].Excerpt != "正确答案是3" {
		t.Fatal(safe)
	}
}
