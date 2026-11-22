package curriculum

import (
	"campusclaw/backend/internal/grading"
	"sort"
	"strings"
	"time"
)

type PublishedQuestion struct {
	Position          int        `json:"position"`
	AssignmentID      int64      `json:"assignment_id"`
	Score             int        `json:"score"`
	Type              string     `json:"type"`
	Stem              string     `json:"stem"`
	Options           []string   `json:"options"`
	KnowledgePointIDs []int64    `json:"knowledge_point_ids"`
	Answer            []string   `json:"answer,omitempty"`
	Explanation       string     `json:"explanation,omitempty"`
	Citations         []Evidence `json:"citations,omitempty"`
}
type PublishedPaper struct {
	ID            int64               `json:"id"`
	Subject       string              `json:"subject"`
	Title         string              `json:"title"`
	TotalScore    int                 `json:"total_score"`
	QuestionCount int                 `json:"question_count"`
	CreatedAt     time.Time           `json:"created_at"`
	Questions     []PublishedQuestion `json:"questions,omitempty"`
}
type PaperAnswer struct {
	AssignmentID int64  `json:"assignment_id"`
	Answer       string `json:"answer"`
}
type PaperAttemptInput struct {
	Answers []PaperAnswer `json:"answers"`
}
type PaperAttempt struct {
	ID            int64                    `json:"id"`
	PaperID       int64                    `json:"paper_id"`
	StudentID     int64                    `json:"student_id"`
	StudentName   string                   `json:"student_name"`
	CreatedAt     time.Time                `json:"created_at"`
	Latest        bool                     `json:"latest"`
	ReviewedCount int                      `json:"reviewed_count"`
	ReviewedScore int                      `json:"reviewed_score"`
	FinalScore    *int                     `json:"final_score"`
	TotalScore    int                      `json:"total_score"`
	Items         []grading.SubmissionView `json:"items"`
}

func canonicalPaperAnswer(q PublishedQuestion, answer string) (string, error) {
	a := strings.TrimSpace(answer)
	switch q.Type {
	case "single_choice", "multiple_choice":
		parts := strings.Split(a, ",")
		if q.Type == "single_choice" && len(parts) != 1 {
			return "", ErrInvalid
		}
		seen := map[string]bool{}
		for _, p := range parts {
			if len(p) != 1 || p[0] < 'A' || int(p[0]-'A') >= len(q.Options) || seen[p] {
				return "", ErrInvalid
			}
			seen[p] = true
		}
		sort.Strings(parts)
		return strings.Join(parts, ","), nil
	case "true_false":
		if a == "true" {
			return "正确", nil
		}
		if a == "false" {
			return "错误", nil
		}
		return "", ErrInvalid
	case "short_answer":
		if len([]rune(a)) < 1 || len([]rune(a)) > 4000 {
			return "", ErrInvalid
		}
		return a, nil
	}
	return "", ErrInvalid
}
func canonicalPaperAnswers(p PublishedPaper, in PaperAttemptInput) ([]PaperAnswer, error) {
	if len(in.Answers) != len(p.Questions) {
		return nil, ErrInvalid
	}
	byID := map[int64]string{}
	for _, v := range in.Answers {
		if _, ok := byID[v.AssignmentID]; ok {
			return nil, ErrInvalid
		}
		byID[v.AssignmentID] = v.Answer
	}
	out := []PaperAnswer{}
	for _, q := range p.Questions {
		a, ok := byID[q.AssignmentID]
		if !ok {
			return nil, ErrInvalid
		}
		answer, e := canonicalPaperAnswer(q, a)
		if e != nil {
			return nil, e
		}
		out = append(out, PaperAnswer{q.AssignmentID, answer})
	}
	return out, nil
}
func paperReference(q QuestionItem) string {
	if q.Type == "true_false" {
		if len(q.Answer) == 1 && q.Answer[0] == "true" {
			return "正确"
		}
		return "错误"
	}
	return strings.Join(q.Answer, ",")
}
