package curriculum

import (
	"campusclaw/backend/internal/auth"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const publishedColumns = `p.id,p.subject,p.title,p.total_score,(SELECT COUNT(*) FROM published_paper_questions q WHERE q.paper_id=p.id),p.created_at`

func scanPublished(row scanner) (PublishedPaper, error) {
	var p PublishedPaper
	e := row.Scan(&p.ID, &p.Subject, &p.Title, &p.TotalScore, &p.QuestionCount, &p.CreatedAt)
	return p, normalize(e)
}
func published(ctx context.Context, q queryer, u auth.User, id int64) (PublishedPaper, error) {
	return scanPublished(q.QueryRowContext(ctx, `SELECT `+publishedColumns+` FROM published_papers p WHERE p.id=? AND p.class_id=?`, id, u.ClassID))
}
func (s *Store) PublishedPapers(ctx context.Context, u auth.User, f PointFilter) ([]PublishedPaper, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT `+publishedColumns+` FROM published_papers p WHERE p.class_id=? AND p.subject=? ORDER BY p.id DESC LIMIT ? OFFSET ?`, u.ClassID, f.Subject, f.Size, int64(f.Page-1)*int64(f.Size))
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []PublishedPaper{}
	for rows.Next() {
		v, e := scanPublished(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func publishedQuestions(ctx context.Context, q queryer, u auth.User, p *PublishedPaper) error {
	rows, e := q.QueryContext(ctx, `SELECT position,assignment_id,item_json FROM published_paper_questions WHERE paper_id=? ORDER BY position`, p.ID)
	if e != nil {
		return e
	}
	items := []PaperItem{}
	p.Questions = []PublishedQuestion{}
	for rows.Next() {
		var v PublishedQuestion
		var raw []byte
		var item PaperItem
		if e = rows.Scan(&v.Position, &v.AssignmentID, &raw); e != nil {
			rows.Close()
			return e
		}
		if e = json.Unmarshal(raw, &item); e != nil {
			rows.Close()
			return e
		}
		v.Score = item.Score
		v.Type = item.Question.Type
		v.Stem = item.Question.Stem
		v.Options = item.Question.Options
		v.KnowledgePointIDs = item.Question.KnowledgePointIDs
		if u.Role == "teacher" {
			v.Answer = item.Question.Answer
			v.Explanation = item.Question.Explanation
		}
		p.Questions = append(p.Questions, v)
		items = append(items, item)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	if u.Role == "teacher" {
		draft, e := resolvePaperEvidence(ctx, q, u, PaperDraft{Items: items})
		if e != nil {
			return e
		}
		for i, item := range draft.Items {
			p.Questions[i].Citations = item.Citations
		}
	}
	return nil
}
func (s *Store) PublishedPaper(ctx context.Context, u auth.User, id int64) (PublishedPaper, error) {
	p, e := published(ctx, s.DB, u, id)
	if e == nil {
		e = publishedQuestions(ctx, s.DB, u, &p)
	}
	return p, e
}
func (s *Store) PublishPaper(ctx context.Context, u auth.User, id int64, version int) (PublishedPaper, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return PublishedPaper{}, e
	}
	defer tx.Rollback()
	if e = validTeacher(ctx, tx, u); e != nil {
		return PublishedPaper{}, e
	}
	draft, e := paper(ctx, tx, u, id, true)
	if e != nil {
		return PublishedPaper{}, e
	}
	var existing int64
	e = tx.QueryRowContext(ctx, `SELECT id FROM published_papers WHERE draft_id=? AND draft_version=? AND class_id=?`, id, version, u.ClassID).Scan(&existing)
	if e == nil {
		tx.Rollback()
		return s.PublishedPaper(ctx, u, existing)
	}
	if e != sql.ErrNoRows {
		return PublishedPaper{}, e
	}
	if draft.Version != version {
		return PublishedPaper{}, ErrConflict
	}
	selections := []PaperSelection{}
	questions := []QuestionItem{}
	for _, item := range draft.Items {
		selections = append(selections, item.PaperSelection)
		questions = append(questions, item.Question)
	}
	if e = validatePaper(draft.Title, selections); e != nil {
		return PublishedPaper{}, e
	}
	if e = checkBankPoints(ctx, tx, u, draft.Subject, questions); e != nil {
		return PublishedPaper{}, ErrConflict
	}
	refs := []Evidence{}
	for _, item := range draft.Items {
		refs = append(refs, item.Citations...)
	}
	if e = lockEvidence(ctx, tx, u, refs); e != nil {
		return PublishedPaper{}, e
	}
	res, e := tx.ExecContext(ctx, `INSERT INTO published_papers(class_id,created_by,draft_id,draft_version,subject,title,total_score) VALUES(?,?,?,?,?,?,?)`, u.ClassID, u.ID, id, version, draft.Subject, draft.Title, draft.TotalScore)
	if e != nil {
		return PublishedPaper{}, e
	}
	pid, e := res.LastInsertId()
	if e != nil {
		return PublishedPaper{}, e
	}
	title := []rune(draft.Title)
	if len(title) > 100 {
		title = title[:100]
	}
	for i, item := range draft.Items {
		stem := item.Question.Stem
		for j, opt := range item.Question.Options {
			stem += fmt.Sprintf("\n%c. %s", 'A'+j, opt)
		}
		res, e = tx.ExecContext(ctx, `INSERT INTO grade_assignments(class_id,created_by,subject,title,question,expected_answer,max_score) VALUES(?,?,?,?,?,?,?)`, u.ClassID, u.ID, draft.Subject, fmt.Sprintf("%s · 第%d题", string(title), i+1), stem, paperReference(item.Question), item.Score)
		if e != nil {
			return PublishedPaper{}, e
		}
		aid, e := res.LastInsertId()
		if e != nil {
			return PublishedPaper{}, e
		}
		raw, _ := json.Marshal(item)
		if _, e = tx.ExecContext(ctx, `INSERT INTO published_paper_questions(paper_id,position,assignment_id,item_json) VALUES(?,?,?,?)`, pid, i+1, aid, string(raw)); e != nil {
			return PublishedPaper{}, e
		}
		for _, point := range item.Question.KnowledgePointIDs {
			if _, e = tx.ExecContext(ctx, `INSERT INTO knowledge_point_assignments(point_id,assignment_id) VALUES(?,?)`, point, aid); e != nil {
				return PublishedPaper{}, e
			}
		}
	}
	if e = tx.Commit(); e != nil {
		return PublishedPaper{}, e
	}
	return s.PublishedPaper(ctx, u, pid)
}

// lockEvidence pins actual source rows while a publication is created.
func lockEvidence(ctx context.Context, tx *sql.Tx, u auth.User, refs []Evidence) error {
	seen := map[int64]bool{}
	for _, ref := range refs {
		if seen[ref.ChunkID] {
			continue
		}
		seen[ref.ChunkID] = true
		live, e := evidence(ctx, tx, u, ref.ChunkID, ref.MaterialID, true)
		if e != nil {
			if errors.Is(e, ErrNotFound) {
				return ErrConflict
			}
			return e
		}
		if live.Excerpt != ref.Excerpt || live.CharStart != ref.CharStart || live.CharEnd != ref.CharEnd {
			return ErrConflict
		}
	}
	return nil
}
func placeholders(n int) string { return strings.TrimRight(strings.Repeat("?,", n), ",") }
