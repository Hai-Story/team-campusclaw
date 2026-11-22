package curriculum

import (
	"strings"
	"time"
)

type PaperSelection struct {
	QuestionID      int64 `json:"question_id"`
	QuestionVersion int   `json:"question_version"`
	Score           int   `json:"score"`
}
type PaperInput struct {
	Subject   string           `json:"subject"`
	Title     string           `json:"title"`
	Items     []PaperSelection `json:"items"`
	ClientKey string           `json:"client_key"`
}

func validatePaper(title string, items []PaperSelection) error {
	if len([]rune(strings.TrimSpace(title))) < 1 || len([]rune(title)) > 120 || len(items) < 1 || len(items) > 50 {
		return ErrInvalid
	}
	seen := map[int64]bool{}
	sum := 0
	for _, v := range items {
		if v.QuestionID < 1 || v.QuestionVersion < 1 || v.Score < 1 || v.Score > 100 || seen[v.QuestionID] {
			return ErrInvalid
		}
		seen[v.QuestionID] = true
		sum += v.Score
	}
	if sum > 1000 {
		return ErrInvalid
	}
	return nil
}
func (in *PaperInput) Validate() error {
	in.Title = strings.TrimSpace(in.Title)
	if !ValidSubject(in.Subject) || !clientKey.MatchString(in.ClientKey) {
		return ErrInvalid
	}
	return validatePaper(in.Title, in.Items)
}
func (in PaperInput) Hash() string { in.ClientKey = ""; return inputHash(in) }

type PaperUpdate struct {
	Title           string           `json:"title"`
	Items           []PaperSelection `json:"items"`
	ExpectedVersion int              `json:"expected_version"`
}

func (in *PaperUpdate) Validate() error {
	in.Title = strings.TrimSpace(in.Title)
	if in.ExpectedVersion < 1 {
		return ErrInvalid
	}
	return validatePaper(in.Title, in.Items)
}

type PaperItem struct {
	PaperSelection
	Question  QuestionItem `json:"question"`
	Citations []Evidence   `json:"citations"`
}
type PaperDraft struct {
	ID         int64       `json:"id"`
	Subject    string      `json:"subject"`
	Title      string      `json:"title"`
	Items      []PaperItem `json:"items"`
	TotalScore int         `json:"total_score"`
	Version    int         `json:"version"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}
