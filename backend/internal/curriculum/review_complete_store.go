package curriculum

import (
	"campusclaw/backend/internal/auth"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

var ErrReviewNotDue = errors.New("复习尚未到期，请按下次复习时间完成")

func lockReviewSource(ctx context.Context, tx *sql.Tx, u auth.User, c ReviewCard) error {
	var id int64
	if c.SourceKind == "favorite" {
		e := tx.QueryRowContext(ctx, `SELECT p.id FROM knowledge_favorites f JOIN knowledge_points p ON p.id=f.point_id WHERE f.student_id=? AND p.id=? AND p.class_id=? AND p.subject=? FOR SHARE`, u.ID, c.SourceID, u.ClassID, c.Subject).Scan(&id)
		if e == sql.ErrNoRows {
			return ErrConflict
		}
		return e
	}
	var assignment int64
	e := tx.QueryRowContext(ctx, `SELECT assignment_id FROM submissions WHERE id=? AND class_id=? AND student_id=?`, c.SourceID, u.ClassID, u.ID).Scan(&assignment)
	if e == sql.ErrNoRows {
		return ErrConflict
	}
	if e != nil {
		return e
	}
	e = tx.QueryRowContext(ctx, `SELECT id FROM grade_assignments WHERE id=? AND class_id=? FOR UPDATE`, assignment, u.ClassID).Scan(&id)
	if e == sql.ErrNoRows {
		return ErrConflict
	}
	if e != nil {
		return e
	}
	e = tx.QueryRowContext(ctx, `SELECT s.id`+effectiveMistake+` WHERE s.id=? AND s.class_id=? AND s.student_id=? AND a.subject=? AND COALESCE(f.correct,r.ai_correct)=FALSE AND `+finalLatest+` FOR SHARE`, c.SourceID, u.ClassID, u.ID, c.Subject).Scan(&id)
	if e == sql.ErrNoRows {
		return ErrConflict
	}
	return e
}
func (s *Store) CompleteReview(ctx context.Context, u auth.User, id int64, in ReviewFeedback, now time.Time) (ReviewCompletion, error) {
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return ReviewCompletion{}, e
	}
	defer tx.Rollback()
	settings, e := initializeReview(ctx, tx, u)
	if e != nil {
		return ReviewCompletion{}, e
	}
	card, e := scanReviewCard(tx.QueryRowContext(ctx, `SELECT `+reviewCardColumns+` FROM review_cards c WHERE c.id=? AND c.student_id=? AND c.class_id=? FOR UPDATE`, id, u.ID, u.ClassID))
	if e != nil {
		return ReviewCompletion{}, e
	}
	hash := inputHash(struct {
		ID    int64
		Input ReviewFeedback
	}{id, in})
	var oldHash string
	e = tx.QueryRowContext(ctx, `SELECT input_hash FROM review_completions WHERE student_id=? AND class_id=? AND client_key=?`, u.ID, u.ClassID, in.ClientKey).Scan(&oldHash)
	get := func() (ReviewCompletion, error) {
		return scanReviewCompletion(tx.QueryRowContext(ctx, `SELECT `+completionColumns+` FROM review_completions WHERE student_id=? AND class_id=? AND client_key=?`, u.ID, u.ClassID, in.ClientKey))
	}
	if e == nil {
		if hash != oldHash {
			return ReviewCompletion{}, ErrConflict
		}
		return get()
	}
	if e != sql.ErrNoRows {
		return ReviewCompletion{}, e
	}
	if card.Version != in.ExpectedVersion {
		return ReviewCompletion{}, ErrConflict
	}
	if e = lockReviewSource(ctx, tx, u, card); e != nil {
		return ReviewCompletion{}, e
	}
	now = now.UTC().Truncate(time.Microsecond)
	if now.Before(card.DueAt) {
		return ReviewCompletion{}, ErrReviewNotDue
	}
	previous := card.Level
	card.Level, card.DueAt, e = reviewSchedule(now, card.Level, in.Feedback, settings.Intervals)
	if e != nil {
		return ReviewCompletion{}, e
	}
	card.Version++
	card.Active = true
	_, e = tx.ExecContext(ctx, `UPDATE review_cards SET level=?,due_at=?,version=?,updated_at=? WHERE id=?`, card.Level, card.DueAt, card.Version, now, card.ID)
	if e != nil {
		return ReviewCompletion{}, e
	}
	raw, _ := json.Marshal(card)
	intervals, _ := json.Marshal(settings.Intervals)
	_, e = tx.ExecContext(ctx, `INSERT INTO review_completions(card_id,student_id,class_id,created_by,client_key,input_hash,feedback,previous_level,level,due_at,completed_at,card_json,intervals_json) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, card.ID, u.ID, u.ClassID, u.ID, in.ClientKey, hash, in.Feedback, previous, card.Level, card.DueAt, now, string(raw), string(intervals))
	if e != nil {
		return ReviewCompletion{}, normalize(e)
	}
	v, e := get()
	if e != nil {
		return v, e
	}
	return v, tx.Commit()
}
