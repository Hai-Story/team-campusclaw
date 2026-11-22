package grading

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"

	"github.com/go-sql-driver/mysql"
)

func contextBackground() context.Context { return context.Background() }

type Store struct{ DB *sql.DB }

func (s *Store) CreateAssignment(ctx context.Context, classID, actor int64, input AssignmentInput) (Assignment, error) {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO grade_assignments(class_id,created_by,subject,title,question,expected_answer,max_score) VALUES(?,?,?,?,?,?,?)`, classID, actor, input.Subject, input.Title, input.Question, input.ExpectedAnswer, input.MaxScore)
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
	err := s.DB.QueryRowContext(ctx, `SELECT a.id,a.class_id,a.subject,a.title,a.question,a.expected_answer,a.max_score,a.created_at,COALESCE(p.paper_id,0),COALESCE(p.position,0) FROM grade_assignments a LEFT JOIN published_paper_questions p ON p.assignment_id=a.id WHERE a.id=? AND a.class_id=?`, id, classID).Scan(&a.ID, &a.ClassID, &a.Subject, &a.Title, &a.Question, &a.ExpectedAnswer, &a.MaxScore, &a.CreatedAt, &a.PaperID, &a.QuestionNumber)
	if err == sql.ErrNoRows {
		err = ErrNotFound
	}
	return a, err
}
func (s *Store) Assignments(ctx context.Context, classID int64, subject string, page, size int) ([]Assignment, error) {
	query := `SELECT id,class_id,subject,title,question,expected_answer,max_score,created_at FROM grade_assignments WHERE class_id=? AND subject=? AND NOT EXISTS(SELECT 1 FROM published_paper_questions p WHERE p.assignment_id=grade_assignments.id)`
	args := []any{classID, subject}
	query += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, size, int64(page-1)*int64(size))
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Assignment{}
	for rows.Next() {
		var a Assignment
		if err = rows.Scan(&a.ID, &a.ClassID, &a.Subject, &a.Title, &a.Question, &a.ExpectedAnswer, &a.MaxScore, &a.CreatedAt); err != nil {
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
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Receipt{}, err
	}
	defer tx.Rollback()
	var id int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM grade_assignments WHERE id=? AND class_id=? FOR SHARE`, assignmentID, classID).Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			err = ErrNotFound
		}
		return Receipt{}, err
	}
	var managed int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM published_paper_questions WHERE assignment_id=?`, assignmentID).Scan(&managed); err != nil {
		return Receipt{}, err
	}
	if managed > 0 {
		return Receipt{}, ErrPaperManaged
	}
	out, err := InsertPending(ctx, tx, classID, student, assignmentID, key, answer)
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
	if err = tx.Commit(); err != nil {
		return Receipt{}, err
	}
	return out, nil
}
func (s *Store) Task(ctx context.Context, classID, actor int64, teacher bool, taskID string, submissionID int64) (Task, error) {
	query := "SELECT " + taskColumns + taskJoins + " WHERE t.class_id=?"
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
	var fields taskFields
	err := s.DB.QueryRowContext(ctx, query, args...).Scan(fields.targets()...)
	if err == sql.ErrNoRows {
		err = ErrNotFound
	}
	if err != nil {
		return Task{}, err
	}
	return fields.value()
}
