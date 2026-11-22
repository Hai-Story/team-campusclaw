package curriculum

import (
	"campusclaw/backend/internal/auth"
	"context"
	"encoding/json"
	"errors"
)

func (s *Store) BankPoints(ctx context.Context, u auth.User, subject string, ids []int64) ([]Point, error) {
	out := []Point{}
	for _, id := range ids {
		p, e := s.Point(ctx, u, id)
		if e != nil {
			return nil, e
		}
		if p.Subject != subject {
			return nil, ErrNotFound
		}
		out = append(out, p)
	}
	return out, nil
}
func checkBankPoints(ctx context.Context, q queryer, u auth.User, subject string, items []QuestionItem) error {
	seen := map[int64]bool{}
	for _, item := range items {
		for _, id := range item.KnowledgePointIDs {
			if seen[id] {
				continue
			}
			var found int64
			if e := q.QueryRowContext(ctx, `SELECT id FROM knowledge_points WHERE id=? AND class_id=? AND subject=? FOR SHARE`, id, u.ClassID, subject).Scan(&found); e != nil {
				return normalize(e)
			}
			seen[id] = true
		}
	}
	return nil
}
func checkBankDuplicates(ctx context.Context, q queryer, u auth.User, subject string, items []QuestionItem, exclude int64) error {
	for _, item := range items {
		var count int
		if e := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM bank_questions WHERE class_id=? AND created_by=? AND subject=? AND stem_hash=? AND id<>?`, u.ClassID, u.ID, subject, item.StemHash(), exclude).Scan(&count); e != nil {
			return e
		}
		if count > 0 {
			return ErrDuplicateQuestion
		}
	}
	return nil
}
func usedQuestionEvidence(items []QuestionItem, refs []Evidence) []Evidence {
	out := []Evidence{}
	seen := map[int]bool{}
	for _, q := range items {
		for _, i := range q.CitationIndices {
			if i > 0 && i <= len(refs) && !seen[i] {
				out = append(out, refs[i-1])
				seen[i] = true
			}
		}
	}
	return out
}

const questionDraftColumns = `id,subject,input_json,questions_json,citations_json,bank_ids_json,status,version,created_at,updated_at`

func scanQuestionDraft(row scanner) (QuestionDraft, error) {
	var v QuestionDraft
	var input, items, cites, ids []byte
	e := row.Scan(&v.ID, &v.Subject, &input, &items, &cites, &ids, &v.Status, &v.Version, &v.CreatedAt, &v.UpdatedAt)
	if e != nil {
		return v, normalize(e)
	}
	for _, p := range []struct {
		raw []byte
		v   any
	}{{input, &v.Input}, {items, &v.Questions}, {cites, &v.Citations}, {ids, &v.BankIDs}} {
		if e = json.Unmarshal(p.raw, p.v); e != nil {
			return v, e
		}
	}
	return v, nil
}
func questionDraft(ctx context.Context, q queryer, u auth.User, id int64, lock bool) (QuestionDraft, error) {
	query := `SELECT ` + questionDraftColumns + ` FROM question_drafts WHERE id=? AND class_id=? AND created_by=?`
	if lock {
		query += ` FOR UPDATE`
	}
	return scanQuestionDraft(q.QueryRowContext(ctx, query, id, u.ClassID, u.ID))
}
func (s *Store) QuestionDraft(ctx context.Context, u auth.User, id int64) (QuestionDraft, error) {
	v, e := questionDraft(ctx, s.DB, u, id, false)
	if e != nil {
		return v, e
	}
	v.Citations, e = s.currentEvidence(ctx, u, v.Citations)
	return v, e
}
func (s *Store) QuestionDraftByKey(ctx context.Context, u auth.User, in BankInput) (QuestionDraft, error) {
	var id int64
	var hash string
	e := s.DB.QueryRowContext(ctx, `SELECT id,input_hash FROM question_drafts WHERE class_id=? AND created_by=? AND client_key=?`, u.ClassID, u.ID, in.ClientKey).Scan(&id, &hash)
	if e != nil {
		return QuestionDraft{}, normalize(e)
	}
	if hash != in.Hash() {
		return QuestionDraft{}, ErrConflict
	}
	return s.QuestionDraft(ctx, u, id)
}
func (s *Store) QuestionDrafts(ctx context.Context, u auth.User, f PointFilter) ([]QuestionDraft, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT `+questionDraftColumns+` FROM question_drafts WHERE class_id=? AND created_by=? AND subject=? AND CAST(questions_json AS CHAR) LIKE ? ORDER BY id DESC LIMIT ? OFFSET ?`, u.ClassID, u.ID, f.Subject, "%"+f.Q+"%", f.Size, int64(f.Page-1)*int64(f.Size))
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []QuestionDraft{}
	for rows.Next() {
		v, e := scanQuestionDraft(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	cache := map[string][]Evidence{}
	for i := range out {
		out[i].Citations, e = resolveReferences(ctx, s.DB, u, out[i].Citations, cache)
		if e != nil {
			return nil, e
		}
	}
	return out, nil
}
func (s *Store) SaveQuestionDraft(ctx context.Context, u auth.User, in BankInput, items []QuestionItem, refs []Evidence) (QuestionDraft, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return QuestionDraft{}, e
	}
	defer tx.Rollback()
	if e = validTeacher(ctx, tx, u); e != nil {
		return QuestionDraft{}, e
	}
	if e = checkBankPoints(ctx, tx, u, in.Subject, items); e != nil {
		return QuestionDraft{}, e
	}
	if e = checkEvidence(ctx, tx, u, usedQuestionEvidence(items, refs)); e != nil {
		return QuestionDraft{}, e
	}
	if e = checkBankDuplicates(ctx, tx, u, in.Subject, items, 0); e != nil {
		return QuestionDraft{}, e
	}
	input, _ := json.Marshal(in)
	questions, _ := json.Marshal(items)
	cites, _ := json.Marshal(refs)
	r, e := tx.ExecContext(ctx, `INSERT INTO question_drafts(class_id,created_by,subject,input_json,questions_json,citations_json,bank_ids_json,client_key,input_hash) VALUES(?,?,?,?,?,?,JSON_ARRAY(),?,?)`, u.ClassID, u.ID, in.Subject, string(input), string(questions), string(cites), in.ClientKey, in.Hash())
	if e != nil {
		tx.Rollback()
		if errors.Is(normalize(e), ErrConflict) {
			return s.QuestionDraftByKey(ctx, u, in)
		}
		return QuestionDraft{}, e
	}
	id, e := r.LastInsertId()
	if e != nil {
		return QuestionDraft{}, e
	}
	if e = tx.Commit(); e != nil {
		return QuestionDraft{}, e
	}
	return s.QuestionDraft(ctx, u, id)
}
func (s *Store) UpdateQuestionDraft(ctx context.Context, u auth.User, id int64, in DraftQuestionsUpdate) (QuestionDraft, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return QuestionDraft{}, e
	}
	defer tx.Rollback()
	v, e := questionDraft(ctx, tx, u, id, true)
	if e != nil {
		return v, e
	}
	if v.Status != "pending" || v.Version != in.ExpectedVersion {
		return v, ErrConflict
	}
	if e = validateQuestions(in.Questions, v.Input, len(v.Citations), false); e != nil {
		return v, e
	}
	if e = checkBankPoints(ctx, tx, u, v.Subject, in.Questions); e != nil {
		return v, e
	}
	if e = checkBankDuplicates(ctx, tx, u, v.Subject, in.Questions, 0); e != nil {
		return v, e
	}
	b, _ := json.Marshal(in.Questions)
	if _, e = tx.ExecContext(ctx, `UPDATE question_drafts SET questions_json=?,version=version+1,updated_at=UTC_TIMESTAMP(6) WHERE id=?`, string(b), id); e != nil {
		return v, e
	}
	v, e = questionDraft(ctx, tx, u, id, false)
	if e != nil {
		return v, e
	}
	v.Citations, e = currentEvidenceWith(ctx, tx, u, v.Citations)
	if e != nil {
		return v, e
	}
	return v, tx.Commit()
}
func (s *Store) BankStems(ctx context.Context, u auth.User, subject string) ([]string, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT stem FROM bank_questions WHERE class_id=? AND created_by=? AND subject=? ORDER BY id DESC LIMIT 100`, u.ClassID, u.ID, subject)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var v string
		if e = rows.Scan(&v); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
