package grading

import (
	"encoding/json"
	"strings"
	"testing"
)

// A1-S2: Unicode character boundaries and score bounds are checked before writes.
func TestAssignmentBoundaries(t *testing.T) {
	valid := AssignmentInput{Title: strings.Repeat("题", 120), Question: "题目", ExpectedAnswer: "2", MaxScore: 100}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, input := range []AssignmentInput{{Title: strings.Repeat("题", 121), Question: "题目", ExpectedAnswer: "2", MaxScore: 100}, {Title: " ", Question: "题", ExpectedAnswer: "2", MaxScore: 10}, {Title: "题", Question: "题", ExpectedAnswer: "2", MaxScore: 101}, {Title: "题", Question: "题", ExpectedAnswer: " ", MaxScore: 10}} {
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
