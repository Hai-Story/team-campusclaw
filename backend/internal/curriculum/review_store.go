package curriculum

import (
	"campusclaw/backend/internal/auth"
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

func reviewActor(ctx context.Context, q queryer, u auth.User, lock bool) error {
	if u.Role != "student" {
		return ErrNotFound
	}
	suffix := ""
	if lock {
		suffix = " FOR SHARE"
	}
	var id int64
	return normalize(q.QueryRowContext(ctx, `SELECT id FROM users WHERE id=? AND class_id=? AND role='student'`+suffix, u.ID, u.ClassID).Scan(&id))
}
func reviewSettings(ctx context.Context, q queryer, u auth.User, lock bool) (ReviewSettings, error) {
	v := defaultReviewSettings()
	var raw []byte
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	e := q.QueryRowContext(ctx, `SELECT intervals_json,version FROM review_settings WHERE student_id=? AND class_id=?`+suffix, u.ID, u.ClassID).Scan(&raw, &v.Version)
	if e == sql.ErrNoRows {
		return v, nil
	}
	if e != nil {
		return v, e
	}
	e = json.Unmarshal(raw, &v.Intervals)
	if e == nil && !validIntervals(v.Intervals) {
		e = ErrInvalid
	}
	return v, e
}
func initializeReview(ctx context.Context, tx *sql.Tx, u auth.User) (ReviewSettings, error) {
	if e := reviewActor(ctx, tx, u, true); e != nil {
		return ReviewSettings{}, e
	}
	_, e := tx.ExecContext(ctx, `INSERT INTO review_settings(student_id,class_id,created_by,intervals_json) VALUES(?,?,?,'[1,3,7,14,30]') ON DUPLICATE KEY UPDATE id=review_settings.id`, u.ID, u.ClassID, u.ID)
	if e != nil {
		return ReviewSettings{}, e
	}
	return reviewSettings(ctx, tx, u, true)
}
func (s *Store) ReviewSettings(ctx context.Context, u auth.User) (ReviewSettings, error) {
	if e := reviewActor(ctx, s.DB, u, false); e != nil {
		return ReviewSettings{}, e
	}
	return reviewSettings(ctx, s.DB, u, false)
}
func (s *Store) SaveReviewSettings(ctx context.Context, u auth.User, in ReviewSettingsInput) (ReviewSettings, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return ReviewSettings{}, e
	}
	defer tx.Rollback()
	old, e := initializeReview(ctx, tx, u)
	if e != nil {
		return old, e
	}
	if old.Version != in.ExpectedVersion {
		return old, ErrConflict
	}
	b, _ := json.Marshal(in.Intervals)
	_, e = tx.ExecContext(ctx, `UPDATE review_settings SET intervals_json=?,version=version+1,updated_at=UTC_TIMESTAMP(6) WHERE student_id=? AND class_id=?`, string(b), u.ID, u.ClassID)
	if e != nil {
		return old, e
	}
	v := ReviewSettings{in.Intervals, old.Version + 1}
	return v, tx.Commit()
}

const effectiveMistake = ` FROM submissions s JOIN grade_assignments a ON a.id=s.assignment_id AND a.class_id=s.class_id JOIN grade_tasks t ON t.submission_id=s.id LEFT JOIN grade_results r ON r.idempotency_key=t.idempotency_key LEFT JOIN grade_reviews f ON f.submission_id=s.id AND f.class_id=s.class_id `
const activeReview = `((c.source_kind='favorite' AND EXISTS(SELECT 1 FROM knowledge_favorites f JOIN knowledge_points p ON p.id=f.point_id WHERE f.student_id=c.student_id AND p.id=c.source_id AND p.class_id=c.class_id AND p.subject=c.subject)) OR (c.source_kind='mistake' AND EXISTS(SELECT 1 ` + effectiveMistake + ` WHERE s.id=c.source_id AND s.student_id=c.student_id AND s.class_id=c.class_id AND a.subject=c.subject AND COALESCE(f.correct,r.ai_correct)=FALSE AND ` + finalLatest + `)))`
const reviewCardColumns = `c.id,c.source_kind,c.source_id,c.subject,c.title,c.body,c.level,c.due_at,c.version,` + activeReview + `,IF(c.source_kind='mistake',COALESCE((SELECT assignment_id FROM submissions WHERE id=c.source_id AND student_id=c.student_id AND class_id=c.class_id),0),0)`

func scanReviewCard(row scanner) (ReviewCard, error) {
	var v ReviewCard
	e := row.Scan(&v.ID, &v.SourceKind, &v.SourceID, &v.Subject, &v.Title, &v.Body, &v.Level, &v.DueAt, &v.Version, &v.Active, &v.AssignmentID)
	return v, normalize(e)
}
func (s *Store) SyncReview(ctx context.Context, u auth.User, now time.Time) error {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = initializeReview(ctx, tx, u); e != nil {
		return e
	}
	insert := `INSERT INTO review_cards(student_id,class_id,created_by,source_kind,source_id,subject,title,body,due_at) `
	update := ` ON DUPLICATE KEY UPDATE updated_at=IF(BINARY review_cards.subject<>BINARY VALUES(subject) OR BINARY review_cards.title<>BINARY VALUES(title) OR BINARY review_cards.body<>BINARY VALUES(body),UTC_TIMESTAMP(6),review_cards.updated_at),version=IF(BINARY review_cards.subject<>BINARY VALUES(subject) OR BINARY review_cards.title<>BINARY VALUES(title) OR BINARY review_cards.body<>BINARY VALUES(body),review_cards.version+1,review_cards.version),subject=VALUES(subject),title=VALUES(title),body=VALUES(body)`
	_, e = tx.ExecContext(ctx, insert+`SELECT ?,?,?,'favorite',p.id,p.subject,p.name,p.description,? FROM knowledge_favorites f JOIN knowledge_points p ON p.id=f.point_id WHERE f.student_id=? AND p.class_id=?`+update, u.ID, u.ClassID, u.ID, now.UTC(), u.ID, u.ClassID)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, insert+`SELECT ?,?,?,'mistake',s.id,a.subject,a.title,a.question,?`+effectiveMistake+` WHERE s.student_id=? AND s.class_id=? AND COALESCE(f.correct,r.ai_correct)=FALSE AND `+finalLatest+update, u.ID, u.ClassID, u.ID, now.UTC(), u.ID, u.ClassID)
	if e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) ReviewReport(ctx context.Context, u auth.User, subject, status string, page, size int, now time.Time) (ReviewReport, error) {
	v := ReviewReport{Subject: subject, Items: []ReviewCard{}, ServerTime: now.UTC(), Timezone: "Asia/Shanghai", Page: page, PageSize: size}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return v, e
	}
	defer tx.Rollback()
	if e = reviewActor(ctx, tx, u, false); e != nil {
		return v, e
	}
	v.Settings, e = reviewSettings(ctx, tx, u, false)
	if e != nil {
		return v, e
	}
	where := ` FROM review_cards c WHERE c.student_id=? AND c.class_id=? AND c.subject=? AND ` + activeReview
	e = tx.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(c.due_at<=?),0)`+where, now.UTC(), u.ID, u.ClassID, subject).Scan(&v.Total, &v.DueCount)
	if e != nil {
		return v, e
	}
	args := []any{u.ID, u.ClassID, subject}
	if status == "due" {
		where += ` AND c.due_at<=?`
		args = append(args, now.UTC())
		v.Total = v.DueCount
	}
	args = append(args, size, int64(page-1)*int64(size))
	rows, e := tx.QueryContext(ctx, `SELECT `+reviewCardColumns+where+` ORDER BY c.due_at,c.id LIMIT ? OFFSET ?`, args...)
	if e != nil {
		return v, e
	}
	for rows.Next() {
		c, err := scanReviewCard(rows)
		if err != nil {
			rows.Close()
			return v, err
		}
		v.Items = append(v.Items, c)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return v, e
	}
	return v, tx.Commit()
}
func (s *Store) ReviewDueCount(ctx context.Context, u auth.User, now time.Time) (int, error) {
	if e := reviewActor(ctx, s.DB, u, false); e != nil {
		return 0, e
	}
	var n int
	e := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM review_cards c WHERE c.student_id=? AND c.class_id=? AND c.due_at<=? AND `+activeReview, u.ID, u.ClassID, now.UTC()).Scan(&n)
	return n, e
}
func scanReviewCompletion(row scanner) (ReviewCompletion, error) {
	var v ReviewCompletion
	var raw []byte
	e := row.Scan(&v.ID, &v.CardID, &v.Feedback, &v.PreviousLevel, &v.Level, &v.DueAt, &v.CompletedAt, &raw)
	if e == nil {
		e = json.Unmarshal(raw, &v.Card)
	}
	return v, normalize(e)
}

const completionColumns = `id,card_id,feedback,previous_level,level,due_at,completed_at,card_json`

func (s *Store) ReviewHistory(ctx context.Context, u auth.User, subject string, page, size int) ([]ReviewCompletion, error) {
	if e := reviewActor(ctx, s.DB, u, false); e != nil {
		return nil, e
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT `+completionColumns+` FROM review_completions WHERE student_id=? AND class_id=? AND JSON_UNQUOTE(JSON_EXTRACT(card_json,'$.subject'))=? ORDER BY id DESC LIMIT ? OFFSET ?`, u.ID, u.ClassID, subject, size, int64(page-1)*int64(size))
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []ReviewCompletion{}
	for rows.Next() {
		v, err := scanReviewCompletion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
