package grading

import (
	"encoding/json"
	"strings"
	"testing"
)

// A1-S2: Unicode character boundaries and score bounds are checked before writes.
func TestAssignmentBoundaries(t *testing.T) {
	valid := AssignmentInput{Subject: "math", Title: strings.Repeat("题", 120), Question: "题目", ExpectedAnswer: "2", MaxScore: 100}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	changes := []func(*AssignmentInput){func(a *AssignmentInput) { a.Subject = "" }, func(a *AssignmentInput) { a.Subject = "unknown" }, func(a *AssignmentInput) { a.Title = strings.Repeat("题", 121) }, func(a *AssignmentInput) { a.Title = " " }, func(a *AssignmentInput) { a.Question = " " }, func(a *AssignmentInput) { a.ExpectedAnswer = " " }, func(a *AssignmentInput) { a.MaxScore = 0 }, func(a *AssignmentInput) { a.MaxScore = 101 }}
	for _, change := range changes {
		input := valid
		change(&input)
		if input.Validate() == nil {
			t.Fatal("invalid assignment accepted")
		}
	}

}

// A2/A4: pending receipts do not accidentally serialize a result or answer.
func TestPendingReceipt(t *testing.T) {
	data, _ := json.Marshal(receipt(1, strings.Repeat("a", 32), "pending"))
	for _, forbidden := range []string{"score", "feedback", "answer"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatal("result leaked before execution")
		}
	}
}

// A8-S2: false and zero are explicit valid values, missing fields are rejected.
func TestReviewInputBoundaries(t *testing.T) {
	zero := 0
	wrong := false
	input := ReviewInput{Score: &zero, Correct: &wrong, ExpectedVersion: &zero}
	if !input.Validate() {
		t.Fatal("zero/false rejected")
	}
	for _, invalid := range []ReviewInput{{Correct: &wrong, ExpectedVersion: &zero}, {Score: &zero, ExpectedVersion: &zero}, {Score: &zero, Correct: &wrong}, {Score: &zero, Correct: &wrong, ExpectedVersion: &zero, Feedback: strings.Repeat("字", 2001)}} {
		if invalid.Validate() {
			t.Fatal("invalid review accepted")
		}
	}
}
