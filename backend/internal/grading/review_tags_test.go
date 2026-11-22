package grading

import "testing"

func TestConfirmedErrorTags(t *testing.T) {
	// D1: only the teacher's valid error labels may enter a review.
	score, version, correct := 0, 0, false
	for _, tags := range [][]string{{"concept"}, {"concept", "method"}, {}} {
		if !(&ReviewInput{Score: &score, Correct: &correct, ExpectedVersion: &version, ErrorTags: tags}).Validate() {
			t.Fatal(tags)
		}
	}
	for _, tags := range [][]string{{"unknown"}, {"concept", "concept"}, {""}} {
		if (&ReviewInput{Score: &score, Correct: &correct, ExpectedVersion: &version, ErrorTags: tags}).Validate() {
			t.Fatal("invalid tags accepted", tags)
		}
	}
	correct = true
	if (&ReviewInput{Score: &score, Correct: &correct, ExpectedVersion: &version, ErrorTags: []string{"concept"}}).Validate() {
		t.Fatal("correct item retained error tags")
	}
}
