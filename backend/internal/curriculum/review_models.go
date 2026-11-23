package curriculum

import "time"

type ReviewSettings struct {
	Intervals []int `json:"intervals"`
	Version   int   `json:"version"`
}

func defaultReviewSettings() ReviewSettings { return ReviewSettings{[]int{1, 3, 7, 14, 30}, 1} }
func validIntervals(v []int) bool {
	if len(v) != 5 {
		return false
	}
	for i, n := range v {
		if n < 1 || n > 365 || (i > 0 && n <= v[i-1]) {
			return false
		}
	}
	return true
}

type ReviewSettingsInput struct {
	Intervals       []int `json:"intervals"`
	ExpectedVersion int   `json:"expected_version"`
}

func (in ReviewSettingsInput) Validate() error {
	if !validIntervals(in.Intervals) || in.ExpectedVersion < 1 {
		return ErrInvalid
	}
	return nil
}

type ReviewFeedback struct {
	Feedback        string `json:"feedback"`
	ClientKey       string `json:"client_key"`
	ExpectedVersion int    `json:"expected_version"`
}

func (in ReviewFeedback) Validate() error {
	if (in.Feedback != "good" && in.Feedback != "again") || !clientKey.MatchString(in.ClientKey) || in.ExpectedVersion < 1 {
		return ErrInvalid
	}
	return nil
}
func reviewSchedule(now time.Time, level int, feedback string, intervals []int) (int, time.Time, error) {
	if level < 0 || level > 5 || !validIntervals(intervals) || (feedback != "good" && feedback != "again") {
		return 0, time.Time{}, ErrInvalid
	}
	next := 0
	days := intervals[0]
	if feedback == "good" {
		next = level + 1
		if next > 5 {
			next = 5
		}
		days = intervals[next-1]
	}
	n := now.In(learningZone)
	due := time.Date(n.Year(), n.Month(), n.Day(), 9, 0, 0, 0, learningZone).AddDate(0, 0, days)
	return next, due.UTC(), nil
}

type ReviewCard struct {
	ID           int64     `json:"id"`
	SourceKind   string    `json:"source_kind"`
	SourceID     int64     `json:"source_id"`
	Subject      string    `json:"subject"`
	Title        string    `json:"title"`
	Body         string    `json:"body"`
	Level        int       `json:"level"`
	DueAt        time.Time `json:"due_at"`
	Version      int       `json:"version"`
	Active       bool      `json:"active"`
	AssignmentID int64     `json:"assignment_id,omitempty"`
}
type ReviewCompletion struct {
	ID            int64      `json:"id"`
	CardID        int64      `json:"card_id"`
	Feedback      string     `json:"feedback"`
	PreviousLevel int        `json:"previous_level"`
	Level         int        `json:"level"`
	DueAt         time.Time  `json:"due_at"`
	CompletedAt   time.Time  `json:"completed_at"`
	Card          ReviewCard `json:"card"`
}
type ReviewReport struct {
	Subject    string         `json:"subject"`
	Items      []ReviewCard   `json:"items"`
	Total      int            `json:"total"`
	DueCount   int            `json:"due_count"`
	Settings   ReviewSettings `json:"settings"`
	ServerTime time.Time      `json:"server_time"`
	Timezone   string         `json:"timezone"`
	Page       int            `json:"page"`
	PageSize   int            `json:"page_size"`
}
