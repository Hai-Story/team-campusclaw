package grading

// RevealsAnswer shares the conservative explanation boundary with learning guidance.
func RevealsAnswer(text, reference string) bool { return revealsAnswer(text, reference) }
