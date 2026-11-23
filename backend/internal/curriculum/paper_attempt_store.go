package curriculum

import (
	"campusclaw/backend/internal/auth"
	"campusclaw/backend/internal/grading"
	"context"
	"database/sql"
	"fmt"
)

func (s *Store) SubmitPaper(ctx context.Context, u auth.User, id int64, key string, in PaperAttemptInput) (PaperAttempt, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return PaperAttempt{}, e
	}
	defer tx.Rollback()
	var actorID int64
	if e = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=? AND class_id=? AND role='student' FOR UPDATE`, u.ID, u.ClassID).Scan(&actorID); e != nil {
		return PaperAttempt{}, normalize(e)
	}
	p, e := published(ctx, tx, u, id)
	if e != nil {
		return PaperAttempt{}, e
	}
	if e = publishedQuestions(ctx, tx, u, &p); e != nil {
		return PaperAttempt{}, e
	}
	answers, e := canonicalPaperAnswers(p, in)
	if e != nil {
		return PaperAttempt{}, e
	}
	hash := inputHash(answers)
	var existing int64
	var oldHash string
	e = tx.QueryRowContext(ctx, `SELECT id,input_hash FROM paper_attempts WHERE paper_id=? AND student_id=? AND client_key=?`, id, u.ID, key).Scan(&existing, &oldHash)
	if e == nil {
		if hash != oldHash {
			return PaperAttempt{}, ErrConflict
		}
		tx.Rollback()
		return s.PaperAttempt(ctx, u, existing)
	}
	if e != sql.ErrNoRows {
		return PaperAttempt{}, e
	}
	res, e := tx.ExecContext(ctx, `INSERT INTO paper_attempts(paper_id,class_id,student_id,client_key,input_hash) VALUES(?,?,?,?,?)`, id, u.ClassID, u.ID, key, hash)
	if e != nil {
		return PaperAttempt{}, e
	}
	attempt, e := res.LastInsertId()
	if e != nil {
		return PaperAttempt{}, e
	}
	for _, a := range answers {
		var aid int64
		if e = tx.QueryRowContext(ctx, `SELECT id FROM grade_assignments WHERE id=? AND class_id=? FOR SHARE`, a.AssignmentID, u.ClassID).Scan(&aid); e != nil {
			return PaperAttempt{}, normalize(e)
		}
		receipt, e := grading.InsertPending(ctx, tx, u.ClassID, u.ID, aid, fmt.Sprintf("paper-%d", attempt), a.Answer)
		if e != nil {
			return PaperAttempt{}, e
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO paper_attempt_submissions(attempt_id,assignment_id,submission_id) VALUES(?,?,?)`, attempt, aid, receipt.SubmissionID); e != nil {
			return PaperAttempt{}, e
		}
	}
	if e = tx.Commit(); e != nil {
		return PaperAttempt{}, e
	}
	return s.PaperAttempt(ctx, u, attempt)
}
func (s *Store) PaperAttempt(ctx context.Context, u auth.User, id int64) (PaperAttempt, error) {
	var v PaperAttempt
	query := `SELECT t.id,t.paper_id,t.student_id,u.username,t.created_at,p.total_score,NOT EXISTS(SELECT 1 FROM paper_attempts n WHERE n.paper_id=t.paper_id AND n.student_id=t.student_id AND n.id>t.id) FROM paper_attempts t JOIN published_papers p ON p.id=t.paper_id AND p.class_id=t.class_id JOIN users u ON u.id=t.student_id AND u.class_id=t.class_id WHERE t.id=? AND t.class_id=?`
	args := []any{id, u.ClassID}
	if u.Role != "teacher" {
		query += ` AND t.student_id=?`
		args = append(args, u.ID)
	}
	e := s.DB.QueryRowContext(ctx, query, args...).Scan(&v.ID, &v.PaperID, &v.StudentID, &v.StudentName, &v.CreatedAt, &v.TotalScore, &v.Latest)
	if e != nil {
		return v, normalize(e)
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT x.submission_id FROM paper_attempt_submissions x JOIN published_paper_questions q ON q.assignment_id=x.assignment_id WHERE x.attempt_id=? ORDER BY q.position`, id)
	if e != nil {
		return v, e
	}
	ids := []int64{}
	for rows.Next() {
		var sid int64
		if e = rows.Scan(&sid); e != nil {
			rows.Close()
			return v, e
		}
		ids = append(ids, sid)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return v, e
	}
	actor := int64(0)
	if u.Role != "teacher" {
		actor = u.ID
	}
	items, e := (&grading.Store{DB: s.DB}).ViewsByIDs(ctx, u.ClassID, actor, ids)
	if e != nil {
		return v, e
	}
	if len(items) != len(ids) {
		return v, ErrNotFound
	}
	byID := map[int64]grading.SubmissionView{}
	for _, item := range items {
		byID[item.ID] = item
	}
	v.Items = []grading.SubmissionView{}
	for _, id := range ids {
		item := byID[id]
		v.Items = append(v.Items, item)
		if f := item.Task.FinalReview; f != nil {
			v.ReviewedCount++
			v.ReviewedScore += f.Score
		}
	}
	if v.ReviewedCount == len(v.Items) {
		score := v.ReviewedScore
		v.FinalScore = &score
	}
	return v, nil
}
func (s *Store) PaperAttempts(ctx context.Context, u auth.User, id int64, page, size int) ([]PaperAttempt, error) {
	if _, e := published(ctx, s.DB, u, id); e != nil {
		return nil, e
	}
	query := `SELECT t.id,t.paper_id,t.student_id,u.username,t.created_at,p.total_score,NOT EXISTS(SELECT 1 FROM paper_attempts n WHERE n.paper_id=t.paper_id AND n.student_id=t.student_id AND n.id>t.id),COUNT(f.submission_id),COALESCE(SUM(f.score),0),COUNT(x.submission_id) FROM paper_attempts t JOIN published_papers p ON p.id=t.paper_id JOIN users u ON u.id=t.student_id AND u.class_id=t.class_id JOIN paper_attempt_submissions x ON x.attempt_id=t.id LEFT JOIN grade_reviews f ON f.submission_id=x.submission_id AND f.class_id=t.class_id WHERE t.paper_id=? AND t.class_id=?`
	args := []any{id, u.ClassID}
	if u.Role != "teacher" {
		query += ` AND t.student_id=?`
		args = append(args, u.ID)
	} else {
		query += ` AND NOT EXISTS(SELECT 1 FROM paper_attempts n WHERE n.paper_id=t.paper_id AND n.student_id=t.student_id AND n.id>t.id)`
	}
	query += ` GROUP BY t.id,t.paper_id,t.student_id,u.username,t.created_at,p.total_score ORDER BY t.id DESC LIMIT ? OFFSET ?`
	args = append(args, size, int64(page-1)*int64(size))
	rows, e := s.DB.QueryContext(ctx, query, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []PaperAttempt{}
	for rows.Next() {
		var v PaperAttempt
		var count int
		if e = rows.Scan(&v.ID, &v.PaperID, &v.StudentID, &v.StudentName, &v.CreatedAt, &v.TotalScore, &v.Latest, &v.ReviewedCount, &v.ReviewedScore, &count); e != nil {
			return nil, e
		}
		v.Items = []grading.SubmissionView{}
		if count == v.ReviewedCount {
			score := v.ReviewedScore
			v.FinalScore = &score
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
