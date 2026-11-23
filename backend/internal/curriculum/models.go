package curriculum

import (
	"errors"
	"strings"
	"time"
)

var ErrInvalid = errors.New("参数非法")
var ErrNotFound = errors.New("资源不存在")
var ErrConflict = errors.New("记录已更新或名称重复，请重新加载")

func ValidSubject(s string) bool {
	switch s {
	case "chinese", "math", "english", "physics", "chemistry", "biology", "politics", "history", "geography":
		return true
	}
	return false
}
func validIDs(ids []int64) bool {
	if len(ids) > 50 {
		return false
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

type PointInput struct {
	Subject         string  `json:"subject"`
	Name            string  `json:"name"`
	Chapter         string  `json:"chapter"`
	Description     string  `json:"description"`
	MaterialIDs     []int64 `json:"material_ids"`
	AssignmentIDs   []int64 `json:"assignment_ids"`
	ExpectedVersion *int    `json:"expected_version,omitempty"`
}

func (in *PointInput) Validate(update bool) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Chapter = strings.TrimSpace(in.Chapter)
	in.Description = strings.TrimSpace(in.Description)
	if !ValidSubject(in.Subject) || len([]rune(in.Name)) < 1 || len([]rune(in.Name)) > 120 || len([]rune(in.Chapter)) > 120 || len([]rune(in.Description)) > 2000 || !validIDs(in.MaterialIDs) || !validIDs(in.AssignmentIDs) {
		return ErrInvalid
	}
	if update && (in.ExpectedVersion == nil || *in.ExpectedVersion < 1) {
		return ErrInvalid
	}
	if !update && in.ExpectedVersion != nil {
		return ErrInvalid
	}
	return nil
}

type Point struct {
	ID            int64     `json:"id"`
	Subject       string    `json:"subject"`
	Name          string    `json:"name"`
	Chapter       string    `json:"chapter"`
	Description   string    `json:"description"`
	Version       int       `json:"version"`
	MaterialIDs   []int64   `json:"material_ids"`
	AssignmentIDs []int64   `json:"assignment_ids"`
	Favorite      bool      `json:"favorite"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
type PointFilter struct {
	Subject, Q string
	Page, Size int
	Favorites  bool
}
