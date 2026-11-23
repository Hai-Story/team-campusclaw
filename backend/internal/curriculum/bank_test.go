package curriculum

import "testing"

// B1/B3: answer semantics, duplicate normalization and deterministic paper totals.
func TestQuestionValidation(t *testing.T) {
	base := QuestionItem{Type: "single_choice", Stem: "比较分数", Options: []string{"整体", "部分"}, Answer: []string{"A"}, Explanation: "核对定义", KnowledgePointIDs: []int64{1}, Difficulty: "easy", CitationIndices: []int{1}}
	in := BankInput{SourceInput: SourceInput{Subject: "math", MaterialIDs: []int64{1}, ClientKey: "case"}, KnowledgePointIDs: []int64{1}, QuestionTypes: []string{"single_choice", "multiple_choice", "true_false", "short_answer"}, Difficulty: "easy", QuestionCount: 1}
	if in.Validate() != nil || validateQuestions([]QuestionItem{base}, in, 1, true) != nil {
		t.Fatal("valid")
	}
	for _, q := range []QuestionItem{func() QuestionItem { q := base; q.Answer = []string{"C"}; return q }(), func() QuestionItem { q := base; q.Options = []string{"相同", " 相同 "}; return q }(), func() QuestionItem { q := base; q.Type = "true_false"; return q }(), func() QuestionItem { q := base; q.CitationIndices = []int{999}; return q }()} {
		if validateQuestions([]QuestionItem{q}, in, 1, true) == nil {
			t.Fatal("bad accepted", q)
		}
	}
	for _, kind := range []string{"multiple_choice", "true_false", "short_answer"} {
		q := base
		q.Type = kind
		if kind == "multiple_choice" {
			q.Answer = []string{"A", "B"}
		} else {
			q.Options = []string{}
			q.Answer = []string{"true"}
			if kind == "short_answer" {
				q.Answer = []string{"简短参考答案"}
			}
		}
		if validateQuestions([]QuestionItem{q}, in, 1, true) != nil {
			t.Fatal(kind)
		}
	}
	if validateQuestions([]QuestionItem{base, base}, in, 1, false) == nil {
		t.Fatal("duplicate")
	}
	if (QuestionItem{Stem: "2-3"}).StemHash() == (QuestionItem{Stem: "2+3"}).StemHash() {
		t.Fatal("math operators collapsed")
	}
	if (QuestionItem{Stem: "Hello world"}).StemHash() != (QuestionItem{Stem: "hello  world"}).StemHash() {
		t.Fatal("normalization")
	}
	if validatePaper("草稿", []PaperSelection{{1, 1, 10}, {2, 1, 20}}) != nil {
		t.Fatal("paper")
	}
	if validatePaper("草稿", []PaperSelection{{1, 1, 10}, {1, 1, 20}}) == nil {
		t.Fatal("duplicate selection")
	}
	many := []PaperSelection{}
	for i := int64(1); i <= 11; i++ {
		many = append(many, PaperSelection{i, 1, 100})
	}
	if validatePaper("草稿", many) == nil {
		t.Fatal("score limit")
	}
}
