package grading

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

var ErrInvalid = errors.New("invalid review")

type ReviewInput struct {
	Score           *int   `json:"score"`
	Correct         *bool  `json:"correct"`
	Feedback        string `json:"feedback"`
	ExpectedVersion *int   `json:"expected_version"`
}

func (in *ReviewInput) Validate() bool {
	in.Feedback = strings.TrimSpace(in.Feedback)
	return in.Score != nil && *in.Score >= 0 && in.Correct != nil && in.ExpectedVersion != nil && *in.ExpectedVersion >= 0 && len([]rune(in.Feedback)) <= 2000
}

type SubmissionView struct {
	ID           int64     `json:"id"`
	AssignmentID int64     `json:"assignment_id"`
	StudentID    int64     `json:"student_id"`
	StudentName  string    `json:"student_name"`
	Answer       string    `json:"answer"`
	SubmittedAt  time.Time `json:"submitted_at"`
	Subject      string    `json:"subject"`
	Title        string    `json:"title"`
	Question     string    `json:"question"`
	MaxScore     int       `json:"max_score"`
	Latest       bool      `json:"latest"`
	Task         Task      `json:"task"`
}

const taskColumns = `t.task_id,t.submission_id,t.status,t.attempts,t.error,t.started_at,t.finished_at,r.ai_correct,r.feedback,f.score,f.max_score,f.correct,f.feedback,f.version,f.updated_at`
const taskJoins = ` FROM grade_tasks t JOIN submissions s ON s.id=t.submission_id JOIN grade_assignments a ON a.id=s.assignment_id AND a.class_id=s.class_id LEFT JOIN grade_results r ON r.idempotency_key=t.idempotency_key LEFT JOIN grade_reviews f ON f.submission_id=s.id AND f.class_id=s.class_id `
const latestSubmission = `NOT EXISTS(SELECT 1 FROM submissions newer WHERE newer.student_id=s.student_id AND newer.assignment_id=s.assignment_id AND newer.id>s.id)`

type taskFields struct {
	task                    Task
	start, finish, reviewed sql.NullTime
	ai, final               sql.NullBool
	feedback, finalFeedback sql.NullString
	score, max, version     sql.NullInt64
}

func (v *taskFields) targets() []any {
	return []any{&v.task.TaskID, &v.task.SubmissionID, &v.task.Status, &v.task.Attempts, &v.task.Error, &v.start, &v.finish, &v.ai, &v.feedback, &v.score, &v.max, &v.final, &v.finalFeedback, &v.version, &v.reviewed}
}
func (v *taskFields) value() Task {
	if v.start.Valid {
		v.task.StartedAt = &v.start.Time
	}
	if v.finish.Valid {
		v.task.FinishedAt = &v.finish.Time
	}
	if v.ai.Valid {
		v.task.Result = &Result{Correct: v.ai.Bool, Feedback: v.feedback.String}
	}
	if v.version.Valid {
		v.task.ReviewVersion = int(v.version.Int64)
		v.task.FinalReview = &Review{Score: int(v.score.Int64), MaxScore: int(v.max.Int64), Correct: v.final.Bool, Feedback: v.finalFeedback.String, Version: int(v.version.Int64), ReviewedAt: v.reviewed.Time}
	}
	return v.task
}
func (s *Store) Views(ctx context.Context, classID, actor, assignment int64, subject string, mistakes, latest bool, page, size int) ([]SubmissionView, error) {
	query := `SELECT s.id,s.assignment_id,s.student_id,u.username,s.answer,s.created_at,a.subject,a.title,a.question,a.max_score,` + latestSubmission + `,` + taskColumns + taskJoins + ` JOIN users u ON u.id=s.student_id WHERE s.class_id=?`
	args := []any{classID}
	if actor > 0 {
		query += ` AND s.student_id=?`
		args = append(args, actor)
	}
	if assignment > 0 {
		query += ` AND s.assignment_id=?`
		args = append(args, assignment)
	}
	if subject != "" {
		query += ` AND a.subject=?`
		args = append(args, subject)
	}
	if mistakes {
		query += ` AND COALESCE(f.correct,r.ai_correct)=FALSE AND ` + latestSubmission
	}
	if latest {
		query += ` AND ` + latestSubmission
	}
	query += ` ORDER BY s.id DESC LIMIT ? OFFSET ?`
	args = append(args, size, int64(page-1)*int64(size))
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []SubmissionView{}
	for rows.Next() {
		var item SubmissionView
		var fields taskFields
		targets := []any{&item.ID, &item.AssignmentID, &item.StudentID, &item.StudentName, &item.Answer, &item.SubmittedAt, &item.Subject, &item.Title, &item.Question, &item.MaxScore, &item.Latest}
		if err = rows.Scan(append(targets, fields.targets()...)...); err != nil {
			return nil, err
		}
		item.Task = fields.value()
		items = append(items, item)
	}
	return items, rows.Err()
}

// Review serializes per submission, then atomically updates the head and appends its history.
func (s *Store) Review(ctx context.Context, classID, teacher, submission int64, in ReviewInput) (Review, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Review{}, err
	}
	defer tx.Rollback()
	var assignment int64
	if err = tx.QueryRowContext(ctx, `SELECT assignment_id FROM submissions WHERE id=? AND class_id=? FOR UPDATE`, submission, classID).Scan(&assignment); err != nil {
		if err == sql.ErrNoRows {
			err = ErrNotFound
		}
		return Review{}, err
	}
	var max int
	if err = tx.QueryRowContext(ctx, `SELECT max_score FROM grade_assignments WHERE id=? AND class_id=?`, assignment, classID).Scan(&max); err != nil {
		return Review{}, err
	}
	var role string
	var currentClass int64
	if err = tx.QueryRowContext(ctx, `SELECT role,class_id FROM users WHERE id=? FOR SHARE`, teacher).Scan(&role, &currentClass); err != nil {
		return Review{}, err
	}
	if role != "teacher" || currentClass != classID {
		return Review{}, ErrNotFound
	}
	if !in.Validate() || *in.Score > max {
		return Review{}, ErrInvalid
	}
	var version int
	err = tx.QueryRowContext(ctx, `SELECT version FROM grade_reviews WHERE submission_id=?`, submission).Scan(&version)
	if err != nil && err != sql.ErrNoRows {
		return Review{}, err
	}
	if version != *in.ExpectedVersion {
		return Review{}, ErrConflict
	}
	version++
	_, err = tx.ExecContext(ctx, `INSERT INTO grade_reviews(submission_id,class_id,created_by,score,max_score,correct,feedback,version) VALUES(?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE created_by=VALUES(created_by),score=VALUES(score),max_score=VALUES(max_score),correct=VALUES(correct),feedback=VALUES(feedback),version=VALUES(version),updated_at=UTC_TIMESTAMP(6)`, submission, classID, teacher, *in.Score, max, *in.Correct, in.Feedback, version)
	if err != nil {
		return Review{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO grade_review_history(submission_id,class_id,created_by,score,max_score,correct,feedback,version) VALUES(?,?,?,?,?,?,?,?)`, submission, classID, teacher, *in.Score, max, *in.Correct, in.Feedback, version)
	if err != nil {
		return Review{}, err
	}
	result := Review{Score: *in.Score, MaxScore: max, Correct: *in.Correct, Feedback: in.Feedback, Version: version}
	if err = tx.QueryRowContext(ctx, `SELECT updated_at FROM grade_reviews WHERE submission_id=?`, submission).Scan(&result.ReviewedAt); err != nil {
		return Review{}, err
	}
	return result, tx.Commit()
}
