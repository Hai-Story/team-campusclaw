package grading

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Busy struct{ Wait time.Duration }

func (b *Busy) Error() string { return "task leased or delayed" }

type Event struct {
	ID                   int64
	TaskID, Kind, Token  string
	Generation, Attempts int
}

func (s *Store) ClaimEvent(ctx context.Context) (*Event, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var e Event
	err = tx.QueryRowContext(ctx, `SELECT id,task_id,kind,generation,publish_attempts FROM grade_outbox WHERE available_at<=UTC_TIMESTAMP(6) AND (state='pending' OR (state='publishing' AND lease_until<UTC_TIMESTAMP(6))) ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&e.ID, &e.TaskID, &e.Kind, &e.Generation, &e.Attempts)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	e.Token, err = newID()
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE grade_outbox SET state='publishing',claim_token=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 15 SECOND),publish_attempts=publish_attempts+1 WHERE id=?`, e.Token, e.ID)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	e.Attempts++
	return &e, nil
}
func (s *Store) Published(ctx context.Context, e Event, ok bool) error {
	if ok {
		_, err := s.DB.ExecContext(ctx, `UPDATE grade_outbox SET state='published',published_at=UTC_TIMESTAMP(6),lease_until=NULL WHERE id=? AND claim_token=?`, e.ID, e.Token)
		return err
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE grade_outbox SET state='pending',available_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),lease_until=NULL WHERE id=? AND claim_token=?`, e.ID, e.Token)
	return err
}

func (s *Store) ClaimTask(ctx context.Context, id string, c Settings) (*Work, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var work Work
	var status string
	var lease sql.NullTime
	var available time.Time
	err = tx.QueryRowContext(ctx, `SELECT task_id,submission_id,class_id,created_by,idempotency_key,status,attempts,lease_until,available_at FROM grade_tasks WHERE task_id=? FOR UPDATE`, id).Scan(&work.TaskID, &work.SubmissionID, &work.ClassID, &work.ActorID, &work.IdempotencyKey, &status, &work.Attempts, &lease, &available)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if status == "succeeded" || status == "failed" {
		return nil, nil
	}
	now := time.Now().UTC()
	if status == "running" && lease.Valid && lease.Time.After(now) {
		return nil, &Busy{time.Until(lease.Time)}
	}
	if available.After(now) {
		return nil, &Busy{time.Until(available)}
	}
	if work.Attempts >= c.MaxAttempts {
		if err = failedEvent(ctx, tx, work, "attempts_exhausted"); err != nil {
			return nil, err
		}
		return nil, tx.Commit()
	}
	var role string
	var classID int64
	err = tx.QueryRowContext(ctx, `SELECT s.answer,a.expected_answer,a.question,a.max_score,a.subject,u.role,u.class_id FROM submissions s JOIN grade_assignments a ON a.id=s.assignment_id AND a.class_id=s.class_id JOIN users u ON u.id=s.student_id WHERE s.id=? AND s.class_id=?`, work.SubmissionID, work.ClassID).Scan(&work.Answer, &work.Expected, &work.Question, &work.MaxScore, &work.Subject, &role, &classID)
	if err != nil {
		return nil, err
	}
	work.Attempts++
	if role != "student" || classID != work.ClassID {
		if err = failedEvent(ctx, tx, work, "permission_changed"); err != nil {
			return nil, err
		}
		return nil, tx.Commit()
	}
	work.RunToken, err = newID()
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE grade_tasks SET status='running',attempts=?,run_token=?,lease_until=?,started_at=UTC_TIMESTAMP(6),finished_at=NULL,error='',updated_at=UTC_TIMESTAMP(6) WHERE task_id=?`, work.Attempts, work.RunToken, now.Add(c.Lease), id)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &work, nil
}

func currentTask(ctx context.Context, tx *sql.Tx, w Work) (bool, error) {
	var status, token string
	err := tx.QueryRowContext(ctx, `SELECT status,COALESCE(run_token,'') FROM grade_tasks WHERE task_id=? FOR UPDATE`, w.TaskID).Scan(&status, &token)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return status == "running" && token == w.RunToken, err
}
func failedEvent(ctx context.Context, tx *sql.Tx, w Work, code string) error {
	if _, err := tx.ExecContext(ctx, `UPDATE grade_tasks SET status='failed',attempts=?,error=?,finished_at=UTC_TIMESTAMP(6),lease_until=NULL,updated_at=UTC_TIMESTAMP(6) WHERE task_id=?`, w.Attempts, code, w.TaskID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO grade_outbox(task_id,kind,generation) VALUES(?,'dead',?) ON DUPLICATE KEY UPDATE task_id=task_id`, w.TaskID, w.Attempts)
	return err
}
func (s *Store) Complete(ctx context.Context, w Work, verdict Result, usage Usage) (bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	ok, err := currentTask(ctx, tx, w)
	if err != nil || !ok {
		return false, err
	}
	var role string
	var classID int64
	if err = tx.QueryRowContext(ctx, `SELECT role,class_id FROM users WHERE id=?`, w.ActorID).Scan(&role, &classID); err != nil {
		return false, err
	}
	if role != "student" || classID != w.ClassID {
		if err = failedEvent(ctx, tx, w, "permission_changed"); err != nil {
			return false, err
		}
		return false, tx.Commit()
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO grade_results(submission_id,class_id,created_by,idempotency_key,score,max_score,ai_correct,feedback,prompt_tokens,completion_tokens) VALUES(?,?,?,?,NULL,?,?,?,?,?) ON DUPLICATE KEY UPDATE idempotency_key=idempotency_key`, w.SubmissionID, w.ClassID, w.ActorID, w.IdempotencyKey, w.MaxScore, verdict.Correct, verdict.Feedback, usage.PromptTokens, usage.CompletionTokens)
	if err != nil {
		return false, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE grade_tasks SET status='succeeded',finished_at=UTC_TIMESTAMP(6),lease_until=NULL,error='',updated_at=UTC_TIMESTAMP(6) WHERE task_id=?`, w.TaskID)
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}
func (s *Store) Fail(ctx context.Context, w Work, failure *GradeError, c Settings, delay time.Duration) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ok, err := currentTask(ctx, tx, w)
	if err != nil || !ok {
		return err
	}
	if !failure.Retry || w.Attempts >= c.MaxAttempts {
		err = failedEvent(ctx, tx, w, failure.Code)
	} else {
		due := time.Now().UTC().Add(delay)
		_, err = tx.ExecContext(ctx, `UPDATE grade_tasks SET status='pending',error=?,available_at=?,lease_until=NULL,updated_at=UTC_TIMESTAMP(6) WHERE task_id=?`, failure.Code, due, w.TaskID)
		if err == nil {
			_, err = tx.ExecContext(ctx, `INSERT INTO grade_outbox(task_id,kind,generation,available_at) VALUES(?,'work',?,?) ON DUPLICATE KEY UPDATE task_id=task_id`, w.TaskID, w.Attempts, due)
		}
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) Metrics(ctx context.Context) (map[string]int, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT status,COUNT(*) FROM grade_tasks GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{"pending": 0, "running": 0, "succeeded": 0, "failed": 0}
	for rows.Next() {
		var name string
		var count int
		if err = rows.Scan(&name, &count); err != nil {
			return nil, err
		}
		counts[name] = count
	}
	return counts, rows.Err()
}
func IsBusy(err error) (time.Duration, bool) {
	var b *Busy
	if errors.As(err, &b) {
		return b.Wait, true
	}
	return 0, false
}
