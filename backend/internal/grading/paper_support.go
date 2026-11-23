package grading

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// InsertPending writes a single question inside the caller's validated transaction.
// The caller must hold the class-scoped assignment lock; commit/outbox publication stay outside.
func InsertPending(ctx context.Context, tx *sql.Tx, classID, student, assignment int64, key, answer string) (Receipt, error) {
	task, e := newID()
	if e != nil {
		return Receipt{}, e
	}
	res, e := tx.ExecContext(ctx, `INSERT INTO submissions(class_id,assignment_id,student_id,created_by,client_key,answer,answer_hash) VALUES(?,?,?,?,?,?,?)`, classID, assignment, student, student, key, answer, answerHash(answer))
	if e != nil {
		return Receipt{}, e
	}
	id, e := res.LastInsertId()
	if e != nil {
		return Receipt{}, e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO grade_tasks(task_id,submission_id,class_id,created_by,idempotency_key) VALUES(?,?,?,?,?)`, task, id, classID, student, fmt.Sprintf("grade:%d:v1", id)); e != nil {
		return Receipt{}, e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO grade_outbox(task_id,kind,generation) VALUES(?,'work',0)`, task); e != nil {
		return Receipt{}, e
	}
	return receipt(id, task, "pending"), nil
}

// ViewsByIDs reads one bounded set in a single statement, so totals share one review snapshot.
func (s *Store) ViewsByIDs(ctx context.Context, classID, actor int64, ids []int64) ([]SubmissionView, error) {
	if len(ids) < 1 || len(ids) > 50 {
		return nil, ErrInvalid
	}
	args := []any{classID}
	for _, id := range ids {
		args = append(args, id)
	}
	query := `SELECT s.id,s.assignment_id,s.student_id,u.username,s.answer,s.created_at,a.subject,a.title,a.question,a.max_score,` + latestSubmission + `,` + taskColumns + taskJoins + ` JOIN users u ON u.id=s.student_id WHERE s.class_id=? AND s.id IN (` + strings.TrimRight(strings.Repeat("?,", len(ids)), ",") + `)`
	if actor > 0 {
		query += ` AND s.student_id=?`
		args = append(args, actor)
	}
	rows, e := s.DB.QueryContext(ctx, query, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []SubmissionView{}
	for rows.Next() {
		var v SubmissionView
		var fields taskFields
		targets := []any{&v.ID, &v.AssignmentID, &v.StudentID, &v.StudentName, &v.Answer, &v.SubmittedAt, &v.Subject, &v.Title, &v.Question, &v.MaxScore, &v.Latest}
		if e = rows.Scan(append(targets, fields.targets()...)...); e != nil {
			return nil, e
		}
		v.Task, e = fields.value()
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
