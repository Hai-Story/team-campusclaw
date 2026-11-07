package grading

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var ErrNotFound = errors.New("resource not found")
var ErrConflict = errors.New("idempotency conflict")
var keyPattern = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)
var taskPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

type Assignment struct {
	ID             int64     `json:"id"`
	ClassID        int64     `json:"-"`
	Title          string    `json:"title"`
	Subject        string    `json:"subject"`
	Question       string    `json:"question"`
	ExpectedAnswer string    `json:"expected_answer,omitempty"`
	MaxScore       int       `json:"max_score"`
	CreatedAt      time.Time `json:"created_at"`
}
type AssignmentInput struct {
	Subject        string `json:"subject"`
	Title          string `json:"title"`
	Question       string `json:"question"`
	ExpectedAnswer string `json:"expected_answer"`
	MaxScore       int    `json:"max_score"`
}

func (a *AssignmentInput) Validate() error {
	if !ValidSubject(a.Subject) {
		return errors.New("请选择有效学科")
	}
	a.Title = strings.TrimSpace(a.Title)
	a.Question = strings.TrimSpace(a.Question)
	a.ExpectedAnswer = strings.TrimSpace(a.ExpectedAnswer)
	for _, field := range []struct {
		name, value string
		max         int
	}{{"title", a.Title, 120}, {"question", a.Question, 4000}, {"expected_answer", a.ExpectedAnswer, 2000}} {
		n := len([]rune(field.value))
		if n < 1 || n > field.max {
			return fmt.Errorf("%s 长度须为1至%d", field.name, field.max)
		}
	}
	if a.MaxScore < 1 || a.MaxScore > 100 {
		return errors.New("max_score 须为1至100的整数")
	}
	return nil
}

func ValidSubject(subject string) bool {
	switch subject {
	case "chinese", "math", "english", "science", "morality":
		return true
	}
	return false
}

type Receipt struct {
	SubmissionID int64  `json:"submission_id"`
	TaskID       string `json:"task_id"`
	Status       string `json:"status"`
	StatusURL    string `json:"status_url"`
}

func receipt(id int64, task, status string) Receipt {
	return Receipt{id, task, status, "/api/tasks/" + task}
}

type Result struct {
	Correct  bool   `json:"correct"`
	Feedback string `json:"feedback"`
}
type Review struct {
	Score      int       `json:"score"`
	MaxScore   int       `json:"max_score"`
	Correct    bool      `json:"correct"`
	Feedback   string    `json:"feedback"`
	Version    int       `json:"version"`
	ReviewedAt time.Time `json:"reviewed_at"`
}
type Task struct {
	TaskID        string     `json:"task_id"`
	SubmissionID  int64      `json:"submission_id"`
	Status        string     `json:"status"`
	Attempts      int        `json:"attempts"`
	Error         string     `json:"error,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
	Result        *Result    `json:"result,omitempty"`
	FinalReview   *Review    `json:"final_review,omitempty"`
	ReviewVersion int        `json:"review_version"`
}

func newID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

type Work struct {
	TaskID, RunToken, IdempotencyKey    string
	SubmissionID, ClassID, ActorID      int64
	Attempts, MaxScore                  int
	Question, Answer, Expected, Subject string
}
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}
type GradeError struct {
	Code  string
	Retry bool
}

func (e *GradeError) Error() string { return e.Code }

type Settings struct {
	AMQPURL, ChatURL, ChatKey, Model string
	Timeout, Lease, RetryBase, Poll  time.Duration
	MaxAttempts                      int
}
