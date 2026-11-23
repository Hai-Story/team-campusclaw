package curriculum

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

const WalkVersion = "walkthrough-v1"

var walkStages = []string{"read", "method", "derive", "check", "completed"}

type WalkCreate struct {
	AssignmentID int64  `json:"assignment_id"`
	ClientKey    string `json:"client_key"`
}

func (in WalkCreate) Validate() error {
	if in.AssignmentID < 1 || !clientKey.MatchString(in.ClientKey) {
		return ErrInvalid
	}
	return nil
}

type WalkInput struct {
	Content         string `json:"content"`
	Action          string `json:"action"`
	ExpectedVersion int    `json:"expected_version"`
	ClientKey       string `json:"client_key"`
}

func (in *WalkInput) Validate() error {
	in.Content = strings.TrimSpace(in.Content)
	if len([]rune(in.Content)) < 1 || len([]rune(in.Content)) > 1000 || in.ExpectedVersion < 1 || !clientKey.MatchString(in.ClientKey) || (in.Action != "discuss" && in.Action != "advance" && in.Action != "finish") {
		return ErrInvalid
	}
	return nil
}
func (in WalkInput) Hash() string {
	b, _ := json.Marshal(in)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func nextWalkStage(stage, action string, discussed bool) (string, error) {
	if stage == "completed" {
		return "", ErrConflict
	}
	if action == "discuss" {
		return stage, nil
	}
	if !discussed {
		return "", ErrConflict
	}
	if action == "finish" {
		if stage == "check" {
			return "completed", nil
		}
		return "", ErrConflict
	}
	if action == "advance" {
		for i, s := range walkStages[:3] {
			if s == stage {
				return walkStages[i+1], nil
			}
		}
	}
	return "", ErrConflict
}

type WalkOutput struct {
	Response     string   `json:"response"`
	Summary      string   `json:"summary"`
	Improvements []string `json:"improvements"`
}

func (o WalkOutput) Validate() error {
	if len([]rune(strings.TrimSpace(o.Response))) < 1 || len([]rune(o.Response)) > 1500 || len([]rune(strings.TrimSpace(o.Summary))) < 1 || len([]rune(o.Summary)) > 1000 || !validTextList(o.Improvements, 5) || generatedCitation.MatchString(o.Response+o.Summary+strings.Join(o.Improvements, "")) {
		return ErrModel
	}
	return nil
}

type WalkTurn struct {
	ID          int64  `json:"id"`
	Content     string `json:"content"`
	Action      string `json:"action"`
	StageBefore string `json:"stage_before"`
	StageAfter  string `json:"stage_after"`
	Status      string `json:"status"`
	WalkOutput
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type Walk struct {
	ID           int64      `json:"id"`
	AssignmentID int64      `json:"assignment_id"`
	Subject      string     `json:"subject"`
	Title        string     `json:"title"`
	Question     string     `json:"question"`
	Reference    string     `json:"-"`
	Stage        string     `json:"stage"`
	Version      int        `json:"version"`
	Summary      string     `json:"summary"`
	Improvements []string   `json:"improvements"`
	Turns        []WalkTurn `json:"turns"`
	Active       bool       `json:"active"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
type WalkRun struct {
	Walk     Walk
	Turn     WalkTurn
	Token    string
	Replayed bool
}
