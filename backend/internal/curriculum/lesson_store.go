package curriculum

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"campusclaw/backend/internal/auth"
	"campusclaw/backend/internal/retrieval"
)

type LessonSummary struct {
	ID        int64     `json:"id"`
	Subject   string    `json:"subject"`
	Title     string    `json:"title"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Store) CheckMaterials(ctx context.Context, u auth.User, ids []int64) error {
	for _, id := range ids {
		var found int64
		if err := s.DB.QueryRowContext(ctx, `SELECT id FROM materials WHERE id=? AND class_id=?`, id, u.ClassID).Scan(&found); err != nil {
			return normalize(err)
		}
	}
	return nil
}
func evidence(ctx context.Context, q queryer, u auth.User, id, material int64, lock bool) (Evidence, error) {
	var e Evidence
	e.ChunkID = id
	e.MaterialID = material
	query := `SELECT m.title,k.chunk_index,k.char_start,k.char_end,k.chunk_text FROM knowledge_chunks k JOIN materials m ON m.id=k.material_id AND m.class_id=k.class_id WHERE k.id=? AND k.material_id=? AND k.class_id=?`
	if lock {
		query += ` FOR SHARE`
	}
	err := q.QueryRowContext(ctx, query, id, material, u.ClassID).Scan(&e.MaterialTitle, &e.ChunkIndex, &e.CharStart, &e.CharEnd, &e.Excerpt)
	e.Available = err == nil
	return e, normalize(err)
}
func (s *Store) Evidence(ctx context.Context, u auth.User, hits []retrieval.Hit) ([]Evidence, error) {
	out := []Evidence{}
	for i, h := range hits {
		e, err := evidence(ctx, s.DB, u, h.ChunkID, h.MaterialID, false)
		if err != nil {
			return nil, err
		}
		e.Index = i + 1
		out = append(out, e)
	}
	return out, nil
}
func (s *Store) currentEvidence(ctx context.Context, u auth.User, refs []Evidence) ([]Evidence, error) {
	return currentEvidenceWith(ctx, s.DB, u, refs)
}
func currentEvidenceWith(ctx context.Context, q queryer, u auth.User, refs []Evidence) ([]Evidence, error) {
	out := append([]Evidence{}, refs...)
	for i, c := range out {
		live, err := evidence(ctx, q, u, c.ChunkID, c.MaterialID, false)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		if err != nil || live.Excerpt != c.Excerpt || live.CharStart != c.CharStart || live.CharEnd != c.CharEnd {
			out[i].Available = false
			out[i].Excerpt = ""
		} else {
			out[i].Available = true
		}
	}
	return out, nil
}

const lessonColumns = `id,subject,title,input_json,body,citations_json,version,created_at,updated_at`

func scanLesson(row scanner) (LessonPlan, error) {
	var p LessonPlan
	var input, refs []byte
	err := row.Scan(&p.ID, &p.Subject, &p.Title, &input, &p.Body, &refs, &p.Version, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return p, normalize(err)
	}
	if err = json.Unmarshal(input, &p.Input); err != nil {
		return p, err
	}
	if err = json.Unmarshal(refs, &p.Citations); err != nil {
		return p, err
	}
	return p, nil
}
func (s *Store) Lesson(ctx context.Context, u auth.User, id int64) (LessonPlan, error) {
	p, err := scanLesson(s.DB.QueryRowContext(ctx, `SELECT `+lessonColumns+` FROM lesson_plans WHERE id=? AND class_id=? AND created_by=?`, id, u.ClassID, u.ID))
	if err != nil {
		return p, err
	}
	p.Citations, err = s.currentEvidence(ctx, u, p.Citations)
	return p, err
}
func (s *Store) LessonByKey(ctx context.Context, u auth.User, in LessonInput) (LessonPlan, error) {
	var id int64
	var hash string
	err := s.DB.QueryRowContext(ctx, `SELECT id,input_hash FROM lesson_plans WHERE class_id=? AND created_by=? AND client_key=?`, u.ClassID, u.ID, in.ClientKey).Scan(&id, &hash)
	if err != nil {
		return LessonPlan{}, normalize(err)
	}
	if hash != in.Hash() {
		return LessonPlan{}, ErrConflict
	}
	return s.Lesson(ctx, u, id)
}
func (s *Store) Lessons(ctx context.Context, u auth.User, f PointFilter) ([]LessonSummary, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,subject,title,version,created_at,updated_at FROM lesson_plans WHERE class_id=? AND created_by=? AND subject=? AND title LIKE ? ORDER BY id DESC LIMIT ? OFFSET ?`, u.ClassID, u.ID, f.Subject, "%"+f.Q+"%", f.Size, int64(f.Page-1)*int64(f.Size))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LessonSummary{}
	for rows.Next() {
		var p LessonSummary
		if err = rows.Scan(&p.ID, &p.Subject, &p.Title, &p.Version, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Store) CreateLesson(ctx context.Context, u auth.User, in LessonInput, body string, refs []Evidence) (LessonPlan, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return LessonPlan{}, err
	}
	defer tx.Rollback()
	var actor int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=? AND class_id=? AND role='teacher' FOR SHARE`, u.ID, u.ClassID).Scan(&actor); err != nil {
		return LessonPlan{}, normalize(err)
	}
	for _, m := range in.MaterialIDs {
		var found int64
		if err = tx.QueryRowContext(ctx, `SELECT id FROM materials WHERE id=? AND class_id=? FOR SHARE`, m, u.ClassID).Scan(&found); err != nil {
			if errors.Is(normalize(err), ErrNotFound) {
				return LessonPlan{}, ErrEvidence
			}
			return LessonPlan{}, err
		}
	}
	for _, ref := range refs {
		live, e := evidence(ctx, tx, u, ref.ChunkID, ref.MaterialID, true)
		if e != nil {
			if errors.Is(e, ErrNotFound) {
				return LessonPlan{}, ErrEvidence
			}
			return LessonPlan{}, e
		}
		if live.Excerpt != ref.Excerpt || live.CharStart != ref.CharStart || live.CharEnd != ref.CharEnd {
			return LessonPlan{}, ErrEvidence
		}
	}
	input, _ := json.Marshal(in)
	cites, _ := json.Marshal(refs)
	title := []rune(in.Chapter + " · 教案")
	if len(title) > 120 {
		title = title[:120]
	}
	r, err := tx.ExecContext(ctx, `INSERT INTO lesson_plans(class_id,created_by,subject,title,input_json,body,citations_json,client_key,input_hash) VALUES(?,?,?,?,?,?,?,?,?)`, u.ClassID, u.ID, in.Subject, string(title), string(input), body, string(cites), in.ClientKey, in.Hash())
	if err != nil {
		if errors.Is(normalize(err), ErrConflict) {
			tx.Rollback()
			return s.LessonByKey(ctx, u, in)
		}
		return LessonPlan{}, err
	}
	id, err := r.LastInsertId()
	if err != nil {
		return LessonPlan{}, err
	}
	p, err := scanLesson(tx.QueryRowContext(ctx, `SELECT `+lessonColumns+` FROM lesson_plans WHERE id=?`, id))
	if err != nil {
		return p, err
	}
	if err = tx.Commit(); err != nil {
		return LessonPlan{}, err
	}
	return p, nil
}
func (s *Store) UpdateLesson(ctx context.Context, u auth.User, id int64, in LessonUpdate, remove bool) (LessonPlan, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return LessonPlan{}, err
	}
	defer tx.Rollback()
	var version int
	if err = tx.QueryRowContext(ctx, `SELECT version FROM lesson_plans WHERE id=? AND class_id=? AND created_by=? FOR UPDATE`, id, u.ClassID, u.ID).Scan(&version); err != nil {
		return LessonPlan{}, normalize(err)
	}
	if version != in.ExpectedVersion {
		return LessonPlan{}, ErrConflict
	}
	if remove {
		if _, err = tx.ExecContext(ctx, `DELETE FROM lesson_plans WHERE id=? AND class_id=? AND created_by=?`, id, u.ClassID, u.ID); err != nil {
			return LessonPlan{}, err
		}
		return LessonPlan{}, tx.Commit()
	}
	if _, err = tx.ExecContext(ctx, `UPDATE lesson_plans SET title=?,body=?,version=version+1,updated_at=UTC_TIMESTAMP(6) WHERE id=? AND class_id=? AND created_by=?`, in.Title, in.Body, id, u.ClassID, u.ID); err != nil {
		return LessonPlan{}, err
	}
	p, err := scanLesson(tx.QueryRowContext(ctx, `SELECT `+lessonColumns+` FROM lesson_plans WHERE id=?`, id))
	if err != nil {
		return p, err
	}
	p.Citations, err = currentEvidenceWith(ctx, tx, u, p.Citations)
	if err != nil {
		return LessonPlan{}, err
	}
	if err = tx.Commit(); err != nil {
		return LessonPlan{}, err
	}
	return p, nil
}
