package curriculum

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrDuplicateQuestion = errors.New("题库已存在相同题干，请调整后重试")

const BankVersion = "question-generation-v1"

var questionTypes = map[string]bool{"single_choice": true, "multiple_choice": true, "true_false": true, "short_answer": true}

func validDifficulty(v string) bool { return v == "easy" || v == "medium" || v == "hard" }

type QuestionError struct {
	Index int
	Why   string
}

func (e *QuestionError) Error() string {
	if e.Index > 0 {
		return fmt.Sprintf("第%d题：%s", e.Index, e.Why)
	}
	return e.Why
}
func (e *QuestionError) Unwrap() error { return ErrInvalid }

type BankInput struct {
	SourceInput
	KnowledgePointIDs []int64  `json:"knowledge_point_ids"`
	QuestionTypes     []string `json:"question_types"`
	Difficulty        string   `json:"difficulty"`
	QuestionCount     int      `json:"question_count"`
}

func (in BankInput) Validate() error {
	if in.SourceInput.Validate() != nil || len(in.KnowledgePointIDs) < 1 || len(in.KnowledgePointIDs) > 10 || !validIDs(in.KnowledgePointIDs) || !validDifficulty(in.Difficulty) || in.QuestionCount < 1 || in.QuestionCount > 10 || len(in.QuestionTypes) < 1 || len(in.QuestionTypes) > 4 {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for _, t := range in.QuestionTypes {
		if !questionTypes[t] || seen[t] {
			return ErrInvalid
		}
		seen[t] = true
	}
	return nil
}
func (in BankInput) Hash() string { in.ClientKey = ""; return inputHash(in) }

type QuestionItem struct {
	Type              string   `json:"type"`
	Stem              string   `json:"stem"`
	Options           []string `json:"options"`
	Answer            []string `json:"answer"`
	Explanation       string   `json:"explanation"`
	KnowledgePointIDs []int64  `json:"knowledge_point_ids"`
	Difficulty        string   `json:"difficulty"`
	CitationIndices   []int    `json:"citation_indices"`
}

func stemKey(stem string) string        { return strings.ToLower(strings.Join(strings.Fields(stem), "")) }
func (q QuestionItem) StemHash() string { return inputHash(stemKey(q.Stem)) }
func (q *QuestionItem) Validate(allowed map[int64]bool, types map[string]bool, difficulty string, refs, index int) error {
	bad := func(s string) error { return &QuestionError{index, s} }
	q.Stem = strings.TrimSpace(q.Stem)
	q.Explanation = strings.TrimSpace(q.Explanation)
	if !types[q.Type] || !questionTypes[q.Type] {
		return bad("请选择有效题型")
	}
	if !validDifficulty(q.Difficulty) || (difficulty != "" && q.Difficulty != difficulty) {
		return bad("难度与设置不一致")
	}
	if len([]rune(q.Stem)) < 1 || len([]rune(q.Stem)) > 2000 {
		return bad("题干须为1至2000字")
	}
	if len([]rune(q.Explanation)) < 1 || len([]rune(q.Explanation)) > 2000 {
		return bad("解析须为1至2000字")
	}
	if len(q.KnowledgePointIDs) < 1 || len(q.KnowledgePointIDs) > 10 || !validIDs(q.KnowledgePointIDs) {
		return bad("请选择1至10个不重复知识点")
	}
	for _, id := range q.KnowledgePointIDs {
		if !allowed[id] {
			return bad("知识点不在指定范围")
		}
	}
	if q.Options == nil || q.Answer == nil {
		return bad("选项和答案必须是数组")
	}
	seen := map[string]bool{}
	for i := range q.Options {
		q.Options[i] = strings.TrimSpace(q.Options[i])
		key := stemKey(q.Options[i])
		if key == "" || len([]rune(q.Options[i])) > 1000 || seen[key] {
			return bad("选项须非空且不重复，每项至多1000字")
		}
		seen[key] = true
	}
	if q.Type == "single_choice" || q.Type == "multiple_choice" {
		if len(q.Options) < 2 || len(q.Options) > 8 {
			return bad("选择题须有2至8个选项")
		}
		minimum := 1
		if q.Type == "multiple_choice" {
			minimum = 2
		}
		if len(q.Answer) < minimum || len(q.Answer) > len(q.Options) || (q.Type == "single_choice" && len(q.Answer) != 1) {
			return bad("答案数量与题型不一致")
		}
		seen = map[string]bool{}
		for _, a := range q.Answer {
			if len(a) != 1 || a[0] < 'A' || int(a[0]-'A') >= len(q.Options) || seen[a] {
				return bad("答案须为不重复的有效选项字母")
			}
			seen[a] = true
		}
	} else {
		if len(q.Options) != 0 || len(q.Answer) != 1 {
			return bad("判断或简答须无选项且有一个答案")
		}
		q.Answer[0] = strings.TrimSpace(q.Answer[0])
		if q.Type == "true_false" {
			if q.Answer[0] != "true" && q.Answer[0] != "false" {
				return bad("判断题答案须为true或false")
			}
		} else if len([]rune(q.Answer[0])) < 1 || len([]rune(q.Answer[0])) > 2000 {
			return bad("简答参考答案须为1至2000字")
		}
	}
	used := map[int]bool{}
	if len(q.CitationIndices) < 1 || len(q.CitationIndices) > refs {
		return bad("请选择真实资料出处")
	}
	for _, i := range q.CitationIndices {
		if i < 1 || i > refs || used[i] {
			return bad("出处编号无效或重复")
		}
		used[i] = true
	}
	text := q.Stem + q.Explanation + strings.Join(q.Options, "") + strings.Join(q.Answer, "")
	for _, m := range generatedCitation.FindAllStringSubmatch(text, -1) {
		found := false
		for i := range used {
			if m[1] == fmt.Sprint(i) {
				found = true
			}
		}
		if !found {
			return bad("正文包含不存在的出处编号")
		}
	}
	return nil
}
func validateQuestions(items []QuestionItem, in BankInput, refs int, exact bool) error {
	if len(items) < 1 || len(items) > 10 || (exact && len(items) != in.QuestionCount) {
		return &QuestionError{0, "题量须为1至10，生成题数须与设置一致"}
	}
	ids := map[int64]bool{}
	for _, id := range in.KnowledgePointIDs {
		ids[id] = true
	}
	types := map[string]bool{}
	for _, kind := range in.QuestionTypes {
		types[kind] = true
	}
	seen := map[string]bool{}
	for i := range items {
		if e := items[i].Validate(ids, types, in.Difficulty, refs, i+1); e != nil {
			return e
		}
		h := items[i].StemHash()
		if seen[h] {
			return &QuestionError{i + 1, "题干与本批其他题目重复"}
		}
		seen[h] = true
	}
	return nil
}

type QuestionDraft struct {
	ID        int64          `json:"id"`
	Subject   string         `json:"subject"`
	Input     BankInput      `json:"input"`
	Questions []QuestionItem `json:"questions"`
	Citations []Evidence     `json:"citations"`
	BankIDs   []int64        `json:"bank_ids"`
	Status    string         `json:"status"`
	Version   int            `json:"version"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}
type BankQuestion struct {
	ID        int64        `json:"id"`
	Subject   string       `json:"subject"`
	Question  QuestionItem `json:"question"`
	Citations []Evidence   `json:"citations"`
	Version   int          `json:"version"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}
type DraftQuestionsUpdate struct {
	Questions       []QuestionItem `json:"questions"`
	ExpectedVersion int            `json:"expected_version"`
}
type BankQuestionUpdate struct {
	Question        QuestionItem `json:"question"`
	ExpectedVersion int          `json:"expected_version"`
	ConfirmReview   bool         `json:"confirm_review"`
}
type BankGeneration struct {
	Status  string         `json:"status"`
	Message string         `json:"message,omitempty"`
	Draft   *QuestionDraft `json:"draft"`
}
