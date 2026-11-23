package curriculum

import (
	"strings"
	"testing"
)

func TestPaperAnswerBoundaries(t *testing.T) {
	for _, tc := range []struct {
		kind, a, want string
		ok            bool
	}{
		{"single_choice", "A", "A", true}, {"single_choice", "A,C", "", false}, {"single_choice", "Z", "", false}, {"multiple_choice", "C,A", "A,C", true}, {"multiple_choice", "A,A", "", false}, {"multiple_choice", "", "", false}, {"multiple_choice", "A", "A", true}, {"true_false", "true", "正确", true}, {"true_false", "false", "错误", true}, {"true_false", "正确", "", false}, {"short_answer", " x ", "x", true}, {"short_answer", strings.Repeat("字", 4001), "", false}} {
		q := PublishedQuestion{Type: tc.kind, Options: []string{"a", "b", "c"}}
		v, e := canonicalPaperAnswer(q, tc.a)
		if (e == nil) != tc.ok || (tc.ok && v != tc.want) {
			t.Fatal(tc, v, e)
		}
	}
	p := PublishedPaper{Questions: []PublishedQuestion{{AssignmentID: 1, Type: "short_answer"}, {AssignmentID: 2, Type: "short_answer"}}}
	for _, in := range []PaperAttemptInput{{}, {Answers: []PaperAnswer{{1, "a"}, {1, "b"}}}, {Answers: []PaperAnswer{{1, "a"}, {3, "b"}}}} {
		if _, e := canonicalPaperAnswers(p, in); e == nil {
			t.Fatal(in)
		}
	}
}
