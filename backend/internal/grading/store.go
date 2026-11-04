package grading

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
)

func contextBackground() context.Context { return context.Background() }

type Store struct{ DB *sql.DB }

func (s *Store) CreateAssignment(ctx context.Context, classID, actor int64, input AssignmentInput) (Assignment, error) {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO grade_assignments(class_id,created_by,title,question,expected_answer,max_score) VALUES(?,?,?,?,?,?)`, classID, actor, input.Title, input.Question, input.ExpectedAnswer, input.MaxScore)
	if err != nil {
		return Assignment{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Assignment{}, err
	}
	return s.Assignment(ctx, classID, id)
}
func (s *Store) Assignment(ctx context.Context, classID, id int64) (Assignment, error) {
	var a Assignment
	err := s.DB.QueryRowContext(ctx, `SELECT id,class_id,title,question,expected_answer,max_score,created_at FROM grade_assignments WHERE id=? AND class_id=?`, id, classID).Scan(&a.ID, &a.ClassID, &a.Title, &a.Question, &a.ExpectedAnswer, &a.MaxScore, &a.CreatedAt)
	if err == sql.ErrNoRows {
		err = ErrNotFound
	}
	return a, err
}
func (s *Store) Assignments(ctx context.Context, classID int64, page, size int) ([]Assignment, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,class_id,title,question,expected_answer,max_score,created_at FROM grade_assignments WHERE class_id=? ORDER BY id DESC LIMIT ? OFFSET ?`, classID, size, int64(page-1)*int64(size))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Assignment{}
	for rows.Next() {
		var a Assignment
		if err = rows.Scan(&a.ID, &a.ClassID, &a.Title, &a.Question, &a.ExpectedAnswer, &a.MaxScore, &a.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}
func answerHash(answer string) string {
	hash := sha256.Sum256([]byte(answer))
	return hex.EncodeToString(hash[:])
}
func (s *Store) Submit(ctx context.Context, classID, student, assignmentID int64, key, answer string) (Receipt, error) {
	task, err := newID()
	if err != nil {
		return Receipt{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Receipt{}, err
	}
	defer tx.Rollback()
	var id int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM grade_assignments WHERE id=? AND class_id=?`, assignmentID, classID).Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			err = ErrNotFound
		}
		return Receipt{}, err
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO submissions(class_id,assignment_id,student_id,created_by,client_key,answer,answer_hash) VALUES(?,?,?,?,?,?,?)`, classID, assignmentID, student, student, key, answer, answerHash(answer))
	if err != nil {
		var duplicate *mysql.MySQLError
		if errors.As(err, &duplicate) && duplicate.Number == 1062 {
			tx.Rollback()
			var hash, status, taskID string
			var subID int64
			err = s.DB.QueryRowContext(ctx, `SELECT s.id,s.answer_hash,t.task_id,t.status FROM submissions s JOIN grade_tasks t ON t.submission_id=s.id WHERE s.class_id=? AND s.student_id=? AND s.assignment_id=? AND s.client_key=?`, classID, student, assignmentID, key).Scan(&subID, &hash, &taskID, &status)
			if err != nil {
				return Receipt{}, err
			}
			if hash != answerHash(answer) {
				return Receipt{}, ErrConflict
			}
			return receipt(subID, taskID, status), nil
		}
		return Receipt{}, err
	}
	id, err = res.LastInsertId()
	if err != nil {
		return Receipt{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO grade_tasks(task_id,submission_id,class_id,created_by,idempotency_key) VALUES(?,?,?,?,?)`, task, id, classID, student, fmt.Sprintf("grade:%d:v1", id)); err != nil {
		return Receipt{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO grade_outbox(task_id,kind,generation) VALUES(?,'work',0)`, task); err != nil {
		return Receipt{}, err
	}
	if err = tx.Commit(); err != nil {
		return Receipt{}, err
	}
	return receipt(id, task, "pending"), nil
}
func (s *Store) Task(ctx context.Context, classID, actor int64, teacher bool, taskID string, submissionID int64) (Task, error) {
	query := `SELECT t.task_id,t.submission_id,t.status,t.attempts,t.error,t.started_at,t.finished_at,r.score,r.max_score,r.feedback FROM grade_tasks t LEFT JOIN grade_results r ON r.idempotency_key=t.idempotency_key WHERE t.class_id=?`
	args := []any{classID}
	if !teacher {
		query += " AND t.created_by=?"
		args = append(args, actor)
	}
	if taskID != "" {
		query += " AND t.task_id=?"
		args = append(args, taskID)
	} else {
		query += " AND t.submission_id=?"
		args = append(args, submissionID)
	}
	var task Task
	var score, max sql.NullInt64
	var feedback sql.NullString
	var start, finish sql.NullTime
	err := s.DB.QueryRowContext(ctx, query, args...).Scan(&task.TaskID, &task.SubmissionID, &task.Status, &task.Attempts, &task.Error, &start, &finish, &score, &max, &feedback)
	if err == sql.ErrNoRows {
		err = ErrNotFound
	}
	if err != nil {
		return Task{}, err
	}
	if start.Valid {
		task.StartedAt = &start.Time
	}
	if finish.Valid {
		task.FinishedAt = &finish.Time
	}
	if score.Valid {
		task.Result = &Result{int(score.Int64), int(max.Int64), feedback.String}
	}
	return task, nil
}
