package curriculum

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"campusclaw/backend/internal/retrieval"
)

const LessonVersion = "lesson-plan-v1"

var clientKey = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)
var generatedCitation = regexp.MustCompile(`\[([0-9]+)\]`)

type LessonInput struct {
	Subject          string  `json:"subject"`
	Chapter          string  `json:"chapter"`
	Objectives       string  `json:"objectives"`
	Periods          int     `json:"periods"`
	MinutesPerPeriod int     `json:"minutes_per_period"`
	MaterialIDs      []int64 `json:"material_ids"`
	ClientKey        string  `json:"client_key"`
}

func (in *LessonInput) Validate() error {
	in.Chapter = strings.TrimSpace(in.Chapter)
	in.Objectives = strings.TrimSpace(in.Objectives)
	if !ValidSubject(in.Subject) || len([]rune(in.Chapter)) < 1 || len([]rune(in.Chapter)) > 120 || len([]rune(in.Objectives)) < 1 || len([]rune(in.Objectives)) > 2000 || in.Periods < 1 || in.Periods > 8 || in.MinutesPerPeriod < 10 || in.MinutesPerPeriod > 90 || len(in.MaterialIDs) < 1 || len(in.MaterialIDs) > 10 || !validIDs(in.MaterialIDs) || !clientKey.MatchString(in.ClientKey) {
		return ErrInvalid
	}
	return nil
}
func (in LessonInput) Hash() string {
	in.ClientKey = ""
	raw, _ := json.Marshal(in)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

type LessonActivity struct {
	Title   string   `json:"title"`
	Minutes int      `json:"minutes"`
	Steps   []string `json:"steps"`
}
type LessonOutput struct {
	Goals           []string         `json:"goals"`
	KeyPoints       []string         `json:"key_points"`
	Difficulties    []string         `json:"difficulties"`
	Activities      []LessonActivity `json:"activities"`
	Resources       []string         `json:"resources"`
	CitationIndices []int            `json:"citation_indices"`
}

func validTextList(v []string, max int) bool {
	if len(v) < 1 || len(v) > max {
		return false
	}
	for _, s := range v {
		if len([]rune(strings.TrimSpace(s))) < 1 || len([]rune(s)) > 600 {
			return false
		}
	}
	return true
}
func (o LessonOutput) Validate(total, candidates int) error {
	if !validTextList(o.Goals, 10) || !validTextList(o.KeyPoints, 10) || !validTextList(o.Difficulties, 10) || !validTextList(o.Resources, 10) || len(o.Activities) < 1 || len(o.Activities) > 20 || len(o.CitationIndices) < 1 || len(o.CitationIndices) > candidates {
		return ErrModel
	}
	minutes := 0
	for _, a := range o.Activities {
		if len([]rune(strings.TrimSpace(a.Title))) < 1 || len([]rune(a.Title)) > 120 || a.Minutes < 1 || a.Minutes > total || !validTextList(a.Steps, 10) {
			return ErrModel
		}
		minutes += a.Minutes
	}
	if minutes != total {
		return ErrModel
	}
	used := map[int]bool{}
	for _, i := range o.CitationIndices {
		if i < 1 || i > candidates || used[i] {
			return ErrModel
		}
		used[i] = true
	}
	for _, match := range generatedCitation.FindAllStringSubmatch(o.Body(), -1) {
		i, _ := strconv.Atoi(match[1])
		if !used[i] {
			return ErrModel
		}
	}
	return nil
}
func (o LessonOutput) Body() string {
	var b strings.Builder
	for _, section := range []struct {
		title string
		items []string
	}{{"教学目标", o.Goals}, {"教学重点", o.KeyPoints}, {"教学难点", o.Difficulties}} {
		fmt.Fprintf(&b, "## %s\n\n", section.title)
		for _, s := range section.items {
			fmt.Fprintf(&b, "- %s\n", s)
		}
		b.WriteString("\n")
	}
	b.WriteString("## 课堂活动与时间分配\n\n")
	for _, a := range o.Activities {
		fmt.Fprintf(&b, "### %s（%d分钟）\n\n", a.Title, a.Minutes)
		for _, s := range a.Steps {
			fmt.Fprintf(&b, "- %s\n", s)
		}
		b.WriteString("\n")
	}
	b.WriteString("## 资源清单\n\n")
	for _, s := range o.Resources {
		fmt.Fprintf(&b, "- %s\n", s)
	}
	b.WriteString("\n## 资料依据\n\n")
	for _, i := range o.CitationIndices {
		fmt.Fprintf(&b, "[%d] ", i)
	}
	return b.String()
}

type Evidence struct {
	retrieval.Citation
	ChunkID   int64 `json:"chunk_id"`
	Available bool  `json:"available"`
}
type LessonPlan struct {
	ID        int64       `json:"id"`
	Subject   string      `json:"subject"`
	Title     string      `json:"title"`
	Input     LessonInput `json:"input"`
	Body      string      `json:"body"`
	Citations []Evidence  `json:"citations"`
	Version   int         `json:"version"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}
type LessonUpdate struct {
	Title           string `json:"title"`
	Body            string `json:"body"`
	ExpectedVersion int    `json:"expected_version"`
}

func (in *LessonUpdate) Validate() error {
	in.Title = strings.TrimSpace(in.Title)
	in.Body = strings.TrimSpace(in.Body)
	if len([]rune(in.Title)) < 1 || len([]rune(in.Title)) > 120 || len([]rune(in.Body)) < 1 || len([]rune(in.Body)) > 20000 || in.ExpectedVersion < 1 {
		return ErrInvalid
	}
	return nil
}
