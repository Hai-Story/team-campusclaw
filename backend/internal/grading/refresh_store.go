package grading

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// LegacyWorks selects only known test feedback or pre-verdict numeric results.
func (s *Store) LegacyWorks(ctx context.Context) ([]Work, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT t.task_id,t.submission_id,t.class_id,t.created_by,t.idempotency_key,t.attempts,a.subject,a.question,a.expected_answer,a.max_score,s.answer FROM grade_tasks t JOIN submissions s ON s.id=t.submission_id JOIN grade_assignments a ON a.id=s.assignment_id AND a.class_id=s.class_id JOIN users u ON u.id=s.student_id AND u.class_id=s.class_id AND u.role='student' JOIN grade_results r ON r.idempotency_key=t.idempotency_key WHERE t.status='succeeded' AND t.idempotency_key NOT LIKE '%:verdict-v3' AND (r.ai_correct IS NULL OR r.feedback LIKE '第%次调用：%') ORDER BY t.submission_id LIMIT 20`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var works []Work
	for rows.Next() {
		var w Work
		if err := rows.Scan(&w.TaskID, &w.SubmissionID, &w.ClassID, &w.ActorID, &w.IdempotencyKey, &w.Attempts, &w.Subject, &w.Question, &w.Expected, &w.MaxScore, &w.Answer); err != nil {
			return nil, err
		}
		works = append(works, w)
	}
	return works, rows.Err()
}

func (s *Store) refreshResult(ctx context.Context, w Work, result Result, usage Usage) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var key, status, role string
	var classID int64
	err = tx.QueryRowContext(ctx, `SELECT t.idempotency_key,t.status,u.role,u.class_id FROM grade_tasks t JOIN users u ON u.id=t.created_by WHERE t.task_id=? FOR UPDATE`, w.TaskID).Scan(&key, &status, &role, &classID)
	if err != nil {
		return err
	}
	if key != w.IdempotencyKey || status != "succeeded" {
		return ErrConflict
	}
	if role != "student" || classID != w.ClassID {
		return ErrNotFound
	}
	newKey := fmt.Sprintf("grade:%d:verdict-v3", w.SubmissionID)
	_, err = tx.ExecContext(ctx, `INSERT INTO grade_results(submission_id,class_id,created_by,idempotency_key,score,max_score,ai_correct,feedback,prompt_tokens,completion_tokens) VALUES(?,?,?,?,NULL,?,?,?,?,?)`, w.SubmissionID, w.ClassID, w.ActorID, newKey, w.MaxScore, result.Correct, result.Feedback, usage.PromptTokens, usage.CompletionTokens)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE grade_tasks SET idempotency_key=?,attempts=attempts+1,finished_at=UTC_TIMESTAMP(6),updated_at=UTC_TIMESTAMP(6) WHERE task_id=?`, newKey, w.TaskID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// RefreshLegacy is an explicit maintenance operation, never a GET side effect.
// The advisory lock prevents concurrent runs; model calls are outside transactions.
func (s *Store) RefreshLegacy(ctx context.Context, c Settings) (int, error) {
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	var locked sql.NullInt64
	if err = conn.QueryRowContext(ctx, `SELECT GET_LOCK('grading-verdict-refresh',0)`).Scan(&locked); err != nil {
		return 0, err
	}
	if !locked.Valid || locked.Int64 != 1 {
		return 0, ErrConflict
	}
	defer conn.ExecContext(context.Background(), `SELECT RELEASE_LOCK('grading-verdict-refresh')`)
	works, err := s.LegacyWorks(ctx)
	if err != nil {
		return 0, err
	}
	updated := 0
	for _, work := range works {
		start := time.Now()
		call, cancel := context.WithTimeout(ctx, c.Timeout)
		result, usage, failure := Feedback(call, c, work)
		cancel()
		status := "succeeded"
		if failure != nil {
			status = "failed"
			err = failure
		} else {
			err = s.refreshResult(ctx, work, result, usage)
		}
		if err != nil {
			status = "failed"
		}
		event("grading", "legacy_refresh", map[string]any{"task_id": work.TaskID, "actor_id": work.ActorID, "skill_id": VerdictVersion, "status": status, "duration_ms": time.Since(start).Milliseconds(), "usage": usage, "model": c.Model})
		if err != nil {
			return updated, errors.New("初判更新未完成，原记录已保留")
		}
		updated++
	}
	return updated, nil
}
