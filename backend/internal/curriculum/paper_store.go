package curriculum

import (
	"campusclaw/backend/internal/auth"
	"context"
	"encoding/json"
	"errors"
	"time"
)

const paperColumns = `id,subject,title,items_json,total_score,version,created_at,updated_at`

func scanPaper(row scanner) (PaperDraft, error) {
	var v PaperDraft
	var items []byte
	e := row.Scan(&v.ID, &v.Subject, &v.Title, &items, &v.TotalScore, &v.Version, &v.CreatedAt, &v.UpdatedAt)
	if e != nil {
		return v, normalize(e)
	}
	e = json.Unmarshal(items, &v.Items)
	return v, e
}
func paper(ctx context.Context, q queryer, u auth.User, id int64, lock bool) (PaperDraft, error) {
	query := `SELECT ` + paperColumns + ` FROM paper_drafts WHERE id=? AND class_id=? AND created_by=?`
	if lock {
		query += ` FOR UPDATE`
	}
	return scanPaper(q.QueryRowContext(ctx, query, id, u.ClassID, u.ID))
}
func (s *Store) Paper(ctx context.Context, u auth.User, id int64) (PaperDraft, error) {
	v, e := paper(ctx, s.DB, u, id, false)
	if e != nil {
		return v, e
	}
	return resolvePaperEvidence(ctx, s.DB, u, v)
}

type PaperSummary struct {
	ID         int64     `json:"id"`
	Subject    string    `json:"subject"`
	Title      string    `json:"title"`
	TotalScore int       `json:"total_score"`
	Version    int       `json:"version"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (s *Store) Papers(ctx context.Context, u auth.User, f PointFilter) ([]PaperSummary, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT id,subject,title,total_score,version,created_at,updated_at FROM paper_drafts WHERE class_id=? AND created_by=? AND subject=? AND title LIKE ? ORDER BY id DESC LIMIT ? OFFSET ?`, u.ClassID, u.ID, f.Subject, "%"+f.Q+"%", f.Size, int64(f.Page-1)*int64(f.Size))
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []PaperSummary{}
	for rows.Next() {
		var v PaperSummary
		if e = rows.Scan(&v.ID, &v.Subject, &v.Title, &v.TotalScore, &v.Version, &v.CreatedAt, &v.UpdatedAt); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func snapshotQuestions(ctx context.Context, q queryer, u auth.User, subject string, selections []PaperSelection, old []PaperItem) ([]PaperItem, int, error) {
	previous := map[int64]PaperItem{}
	for _, v := range old {
		previous[v.QuestionID] = v
	}
	out := []PaperItem{}
	total := 0
	for _, selected := range selections {
		if item, ok := previous[selected.QuestionID]; ok && item.QuestionVersion == selected.QuestionVersion {
			item.Score = selected.Score
			out = append(out, item)
		} else {
			bank, e := bankQuestion(ctx, q, u, selected.QuestionID, " FOR SHARE")
			if e != nil {
				return nil, 0, e
			}
			if bank.Subject != subject {
				return nil, 0, ErrNotFound
			}
			if bank.Version != selected.QuestionVersion {
				return nil, 0, ErrConflict
			}
			out = append(out, PaperItem{PaperSelection: selected, Question: bank.Question, Citations: bank.Citations})
		}
		total += selected.Score
	}
	return out, total, nil
}
func (s *Store) paperByKey(ctx context.Context, u auth.User, in PaperInput) (PaperDraft, error) {
	var id int64
	var hash string
	e := s.DB.QueryRowContext(ctx, `SELECT id,input_hash FROM paper_drafts WHERE class_id=? AND created_by=? AND client_key=?`, u.ClassID, u.ID, in.ClientKey).Scan(&id, &hash)
	if e != nil {
		return PaperDraft{}, normalize(e)
	}
	if hash != in.Hash() {
		return PaperDraft{}, ErrConflict
	}
	return s.Paper(ctx, u, id)
}
func (s *Store) CreatePaper(ctx context.Context, u auth.User, in PaperInput) (PaperDraft, error) {
	if v, e := s.paperByKey(ctx, u, in); e == nil {
		return v, nil
	} else if !errors.Is(e, ErrNotFound) {
		return v, e
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return PaperDraft{}, e
	}
	defer tx.Rollback()
	if e = validTeacher(ctx, tx, u); e != nil {
		return PaperDraft{}, e
	}
	items, total, e := snapshotQuestions(ctx, tx, u, in.Subject, in.Items, nil)
	if e != nil {
		return PaperDraft{}, e
	}
	b, _ := json.Marshal(items)
	r, e := tx.ExecContext(ctx, `INSERT INTO paper_drafts(class_id,created_by,subject,title,items_json,total_score,client_key,input_hash) VALUES(?,?,?,?,?,?,?,?)`, u.ClassID, u.ID, in.Subject, in.Title, string(b), total, in.ClientKey, in.Hash())
	if e != nil {
		tx.Rollback()
		if errors.Is(normalize(e), ErrConflict) {
			return s.paperByKey(ctx, u, in)
		}
		return PaperDraft{}, e
	}
	id, e := r.LastInsertId()
	if e != nil {
		return PaperDraft{}, e
	}
	v, e := paper(ctx, tx, u, id, false)
	if e != nil {
		return v, e
	}
	v, e = resolvePaperEvidence(ctx, tx, u, v)
	if e != nil {
		return v, e
	}
	return v, tx.Commit()
}
func (s *Store) UpdatePaper(ctx context.Context, u auth.User, id int64, in PaperUpdate) (PaperDraft, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return PaperDraft{}, e
	}
	defer tx.Rollback()
	v, e := paper(ctx, tx, u, id, true)
	if e != nil {
		return v, e
	}
	if v.Version != in.ExpectedVersion {
		return v, ErrConflict
	}
	items, total, e := snapshotQuestions(ctx, tx, u, v.Subject, in.Items, v.Items)
	if e != nil {
		return v, e
	}
	b, _ := json.Marshal(items)
	if _, e = tx.ExecContext(ctx, `UPDATE paper_drafts SET title=?,items_json=?,total_score=?,version=version+1,updated_at=UTC_TIMESTAMP(6) WHERE id=?`, in.Title, string(b), total, id); e != nil {
		return v, e
	}
	v, e = paper(ctx, tx, u, id, false)
	if e != nil {
		return v, e
	}
	v, e = resolvePaperEvidence(ctx, tx, u, v)
	if e != nil {
		return v, e
	}
	return v, tx.Commit()
}

func resolvePaperEvidence(ctx context.Context, q queryer, u auth.User, v PaperDraft) (PaperDraft, error) {
	var e error
	cache := map[string][]Evidence{}
	for i := range v.Items {
		b, _ := json.Marshal(v.Items[i].Citations)
		key := string(b)
		refs, ok := cache[key]
		if !ok {
			refs, e = currentEvidenceWith(ctx, q, u, v.Items[i].Citations)
			if e != nil {
				return v, e
			}
			cache[key] = refs
		}
		v.Items[i].Citations = refs
	}
	return v, nil
}
