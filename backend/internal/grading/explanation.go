package grading

import (
	"context"
	"encoding/json"
	"io"
	"regexp"
	"strconv"
	"strings"
)

const ExplanationVersion = "mistake-explanation-v1"

var numberToken = regexp.MustCompile(`[-+−]?\d+(?:\.\d+)?`)
var finalAnswerPhrase = regexp.MustCompile(`正确答案|最终答案|答案[为:：]|最终结果[是为:：]`)

func revealsAnswer(text, reference string) bool {
	compact := func(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), "")) }
	ref, content := compact(reference), compact(text)
	if finalAnswerPhrase.MatchString(content) || strings.Contains(strings.ReplaceAll(content, "答案是否", ""), "答案是") || strings.Contains(content, "不直接给答案") || strings.Contains(content, "只给思路") {
		return true
	}
	if ref == "" {
		return false
	}
	if numberToken.FindString(ref) == ref {
		gold, _ := strconv.ParseFloat(strings.ReplaceAll(ref, "−", "-"), 64)
		for _, token := range numberToken.FindAllString(content, -1) {
			n, err := strconv.ParseFloat(strings.ReplaceAll(token, "−", "-"), 64)
			if err == nil && n == gold {
				return true
			}
		}
		return false
	}
	if len([]rune(ref)) >= 2 {
		return strings.Contains(content, ref)
	}
	if strings.Contains("abcd", ref) {
		return regexp.MustCompile(`(?i)(^|[^a-z])` + regexp.QuoteMeta(ref) + `([^a-z]|$)`).MatchString(text)
	}
	return false
}

func Explain(ctx context.Context, c Settings, work Work) (string, Usage, *GradeError) {
	prompt, _ := promptFiles.ReadFile("prompts/explanation-v1.txt")
	content, usage, failure := completion(ctx, c, "explain-"+strconv.FormatInt(work.SubmissionID, 10), string(prompt), map[string]any{"subject": work.Subject, "question": work.Question, "answer": work.Answer}, 768)
	if failure != nil {
		return "", usage, failure
	}
	var result struct {
		Explanation string `json:"explanation"`
	}
	d := json.NewDecoder(strings.NewReader(content))
	d.DisallowUnknownFields()
	if d.Decode(&result) != nil {
		return "", usage, &GradeError{"gateway_invalid_response", false}
	}
	var extra any
	result.Explanation = strings.TrimSpace(result.Explanation)
	if d.Decode(&extra) != io.EOF || len([]rune(result.Explanation)) < 1 || len([]rune(result.Explanation)) > 1500 || revealsAnswer(result.Explanation, work.Expected) {
		return "", usage, &GradeError{"explanation_invalid_response", false}
	}
	return result.Explanation, usage, nil
}
