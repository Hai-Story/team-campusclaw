package curriculum

import (
	"campusclaw/backend/internal/auth"
	"context"
	"encoding/json"
	"errors"
)

func (s *Store) ApproveQuestionDraft(ctx context.Context, u auth.User, id int64, version int) (QuestionDraft, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return QuestionDraft{}, e
	}
	defer tx.Rollback()
	v, e := questionDraft(ctx, tx, u, id, true)
	if e != nil {
		return v, e
	}
	if v.Status == "approved" && (version == v.Version || version == v.Version-1) {
		return v, nil
	}
	if v.Status != "pending" || v.Version != version {
		return v, ErrConflict
	}
	if e = validTeacher(ctx, tx, u); e != nil {
		return v, e
	}
	if e = validateQuestions(v.Questions, v.Input, len(v.Citations), false); e != nil {
		return v, e
	}
	if e = checkBankPoints(ctx, tx, u, v.Subject, v.Questions); e != nil {
		return v, e
	}
	if e = checkEvidence(ctx, tx, u, usedQuestionEvidence(v.Questions, v.Citations)); e != nil {
		return v, e
	}
	if e = checkBankDuplicates(ctx, tx, u, v.Subject, v.Questions, 0); e != nil {
		return v, e
	}
	ids := []int64{}
	cites, _ := json.Marshal(v.Citations)
	for _, q := range v.Questions {
		b, _ := json.Marshal(q)
		r, e := tx.ExecContext(ctx, `INSERT INTO bank_questions(class_id,created_by,subject,origin_draft_id,question_json,citations_json,stem,stem_hash) VALUES(?,?,?,?,?,?,?,?)`, u.ClassID, u.ID, v.Subject, id, string(b), string(cites), q.Stem, q.StemHash())
		if e != nil {
			if errors.Is(normalize(e), ErrConflict) {
				return v, ErrDuplicateQuestion
			}
			return v, e
		}
		qid, e := r.LastInsertId()
		if e != nil {
			return v, e
		}
		ids = append(ids, qid)
	}
	b, _ := json.Marshal(ids)
	if _, e = tx.ExecContext(ctx, `UPDATE question_drafts SET status='approved',version=version+1,bank_ids_json=?,updated_at=UTC_TIMESTAMP(6) WHERE id=?`, string(b), id); e != nil {
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

const bankQuestionColumns = `id,subject,question_json,citations_json,version,created_at,updated_at`

func scanBankQuestion(row scanner) (BankQuestion, error) {
	var v BankQuestion
	var q, cites []byte
	e := row.Scan(&v.ID, &v.Subject, &q, &cites, &v.Version, &v.CreatedAt, &v.UpdatedAt)
	if e != nil {
		return v, normalize(e)
	}
	if e = json.Unmarshal(q, &v.Question); e != nil {
		return v, e
	}
	e = json.Unmarshal(cites, &v.Citations)
	return v, e
}
func bankQuestion(ctx context.Context, q queryer, u auth.User, id int64, lock string) (BankQuestion, error) {
	return scanBankQuestion(q.QueryRowContext(ctx, `SELECT `+bankQuestionColumns+` FROM bank_questions WHERE id=? AND class_id=? AND created_by=?`+lock, id, u.ClassID, u.ID))
}
func (s *Store) BankQuestion(ctx context.Context, u auth.User, id int64) (BankQuestion, error) {
	v, e := bankQuestion(ctx, s.DB, u, id, "")
	if e != nil {
		return v, e
	}
	v.Citations, e = s.currentEvidence(ctx, u, v.Citations)
	return v, e
}
func (s *Store) BankQuestions(ctx context.Context, u auth.User, f PointFilter) ([]BankQuestion, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT `+bankQuestionColumns+` FROM bank_questions WHERE class_id=? AND created_by=? AND subject=? AND stem LIKE ? ORDER BY id DESC LIMIT ? OFFSET ?`, u.ClassID, u.ID, f.Subject, "%"+f.Q+"%", f.Size, int64(f.Page-1)*int64(f.Size))
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []BankQuestion{}
	for rows.Next() {
		v, e := scanBankQuestion(rows)
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
func (s *Store) UpdateBankQuestion(ctx context.Context, u auth.User, id int64, in BankQuestionUpdate) (BankQuestion, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return BankQuestion{}, e
	}
	defer tx.Rollback()
	v, e := bankQuestion(ctx, tx, u, id, " FOR UPDATE")
	if e != nil {
		return v, e
	}
	if v.Version != in.ExpectedVersion {
		return v, ErrConflict
	}
	allowed := map[int64]bool{}
	for _, pid := range in.Question.KnowledgePointIDs {
		allowed[pid] = true
	}
	if e = in.Question.Validate(allowed, questionTypes, "", len(v.Citations), 1); e != nil {
		return v, e
	}
	if e = checkBankPoints(ctx, tx, u, v.Subject, []QuestionItem{in.Question}); e != nil {
		return v, e
	}
	if e = checkBankDuplicates(ctx, tx, u, v.Subject, []QuestionItem{in.Question}, id); e != nil {
		return v, e
	}
	b, _ := json.Marshal(in.Question)
	_, e = tx.ExecContext(ctx, `UPDATE bank_questions SET question_json=?,stem=?,stem_hash=?,version=version+1,updated_at=UTC_TIMESTAMP(6) WHERE id=?`, string(b), in.Question.Stem, in.Question.StemHash(), id)
	if e != nil {
		if errors.Is(normalize(e), ErrConflict) {
			return v, ErrDuplicateQuestion
		}
		return v, e
	}
	v, e = bankQuestion(ctx, tx, u, id, "")
	if e != nil {
		return v, e
	}
	v.Citations, e = currentEvidenceWith(ctx, tx, u, v.Citations)
	if e != nil {
		return v, e
	}
	return v, tx.Commit()
}
func (s *Store) DeleteBankQuestion(ctx context.Context, u auth.User, id int64, expected int) error {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	v, e := bankQuestion(ctx, tx, u, id, " FOR UPDATE")
	if e != nil {
		return e
	}
	if v.Version != expected {
		return ErrConflict
	}
	if _, e = tx.ExecContext(ctx, `DELETE FROM bank_questions WHERE id=?`, id); e != nil {
		return e
	}
	return tx.Commit()
}
