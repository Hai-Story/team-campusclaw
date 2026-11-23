package curriculum

import (
	"campusclaw/backend/internal/homeworknotice"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"campusclaw/backend/internal/auth"
	"campusclaw/backend/internal/grading"
	"campusclaw/backend/internal/subjects"
)

type JuniorHomeworkResult struct{ CreatedSingle, CreatedPapers, SkippedSingle, SkippedPapers int }
type juniorResources struct {
	BankIDs  []int64 `json:"bank_ids"`
	SingleID int64   `json:"homework_single_id"`
	PaperID  int64   `json:"homework_paper_id"`
}

func juniorLedger(ctx context.Context, q queryer, u auth.User, key, lock string) (juniorResources, error) {
	var raw []byte
	var v juniorResources
	e := q.QueryRowContext(ctx, `SELECT resources_json FROM curriculum_seed_items WHERE class_id=? AND created_by=? AND batch=? AND unit_key=? AND status='complete'`+lock, u.ClassID, u.ID, JuniorBatch, key).Scan(&raw)
	if e != nil {
		return v, normalize(e)
	}
	e = json.Unmarshal(raw, &v)
	if e == nil && len(v.BankIDs) != 4 {
		return v, ErrConflict
	}
	return v, e
}

// ImportJuniorHomework only copies already stored questions; it never calls a model.
func (s *Store) ImportJuniorHomework(ctx context.Context, u auth.User) (JuniorHomeworkResult, error) {
	out := JuniorHomeworkResult{}
	units, e := JuniorUnits()
	if e != nil {
		return out, e
	}
	conn, e := s.DB.Conn(ctx)
	if e != nil {
		return out, e
	}
	defer conn.Close()
	lock := fmt.Sprintf("junior-homework-%d-%d", u.ClassID, u.ID)
	var held int
	if e = conn.QueryRowContext(ctx, "SELECT GET_LOCK(?,0)", lock).Scan(&held); e != nil || held != 1 {
		return out, ErrBusy
	}
	defer conn.ExecContext(context.WithoutCancel(ctx), "SELECT RELEASE_LOCK(?)", lock)
	for _, unit := range units {
		if !strings.HasSuffix(unit.Key, "-01") && !strings.HasSuffix(unit.Key, "-02") {
			continue
		}
		skip, err := s.juniorSingle(ctx, u, unit)
		if err != nil {
			return out, fmt.Errorf("%s单题未完成: %w", unit.Key, err)
		}
		if skip {
			out.SkippedSingle++
		} else {
			out.CreatedSingle++
		}
		if !strings.HasSuffix(unit.Key, "-01") {
			continue
		}
		skip, err = s.juniorPaper(ctx, u, unit)
		if err != nil {
			return out, fmt.Errorf("%s多题未完成: %w", unit.Key, err)
		}
		if skip {
			out.SkippedPapers++
		} else {
			out.CreatedPapers++
		}
	}
	return out, nil
}
func (s *Store) juniorSingle(ctx context.Context, u auth.User, unit JuniorUnit) (bool, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return false, e
	}
	defer tx.Rollback()
	if e = validTeacher(ctx, tx, u); e != nil {
		return false, e
	}
	ledger, e := juniorLedger(ctx, tx, u, unit.Key, " FOR UPDATE")
	if e != nil {
		return false, e
	}
	if ledger.SingleID > 0 {
		return true, nil
	}
	bank, e := bankQuestion(ctx, tx, u, ledger.BankIDs[3], " FOR SHARE")
	if e != nil {
		return false, e
	}
	q := bank.Question
	if bank.Subject != unit.Subject || q.Type != "short_answer" {
		return false, ErrConflict
	}
	if e = checkBankPoints(ctx, tx, u, unit.Subject, []QuestionItem{q}); e != nil {
		return false, e
	}
	if e = lockEvidence(ctx, tx, u, bank.Citations); e != nil {
		return false, e
	}
	a := grading.AssignmentInput{Subject: unit.Subject, Title: "【示例】" + subjects.Name(unit.Subject) + " · " + unit.Title + "练习", Question: q.Stem, ExpectedAnswer: paperReference(q), MaxScore: 10}
	if e = a.Validate(); e != nil {
		return false, e
	}
	r, e := tx.ExecContext(ctx, `INSERT INTO grade_assignments(class_id,created_by,subject,title,question,expected_answer,max_score) VALUES(?,?,?,?,?,?,?)`, u.ClassID, u.ID, a.Subject, a.Title, a.Question, a.ExpectedAnswer, a.MaxScore)
	if e != nil {
		return false, e
	}
	id, e := r.LastInsertId()
	if e != nil {
		return false, e
	}
	for _, point := range q.KnowledgePointIDs {
		if _, e = tx.ExecContext(ctx, `INSERT INTO knowledge_point_assignments(point_id,assignment_id) VALUES(?,?)`, point, id); e != nil {
			return false, e
		}
	}
	if _, e = tx.ExecContext(ctx, `UPDATE curriculum_seed_items SET resources_json=JSON_SET(resources_json,'$.homework_single_id',?) WHERE class_id=? AND created_by=? AND batch=? AND unit_key=?`, id, u.ClassID, u.ID, JuniorBatch, unit.Key); e != nil {
		return false, e
	}
	if e = homeworknotice.Enqueue(ctx, tx, u.ClassID, u.ID, id, 0); e != nil {
		return false, e
	}
	return false, tx.Commit()
}
func (s *Store) juniorPaper(ctx context.Context, u auth.User, unit JuniorUnit) (bool, error) {
	ledger, e := juniorLedger(ctx, s.DB, u, unit.Key, "")
	if e != nil {
		return false, e
	}
	if ledger.PaperID > 0 {
		return true, nil
	}
	key := "junior-homework-v1-" + unit.Subject
	var id int64
	e = s.DB.QueryRowContext(ctx, `SELECT id FROM paper_drafts WHERE class_id=? AND created_by=? AND client_key=?`, u.ClassID, u.ID, key).Scan(&id)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return false, e
	}
	var draft PaperDraft
	if errors.Is(e, sql.ErrNoRows) {
		in := PaperInput{Subject: unit.Subject, Title: "【示例】" + subjects.Name(unit.Subject) + " · " + unit.Title + "综合练习", ClientKey: key}
		for _, bid := range ledger.BankIDs {
			q, err := s.BankQuestion(ctx, u, bid)
			if err != nil {
				return false, err
			}
			in.Items = append(in.Items, PaperSelection{QuestionID: bid, QuestionVersion: q.Version, Score: 10})
		}
		if e = in.Validate(); e != nil {
			return false, e
		}
		draft, e = s.CreatePaper(ctx, u, in)
	} else {
		draft, e = s.Paper(ctx, u, id)
	}
	if e != nil {
		return false, e
	}
	var pid int64
	e = s.DB.QueryRowContext(ctx, `SELECT id FROM published_papers WHERE class_id=? AND draft_id=? ORDER BY id LIMIT 1`, u.ClassID, draft.ID).Scan(&pid)
	if errors.Is(e, sql.ErrNoRows) {
		if draft.Version != 1 {
			return false, ErrConflict
		}
		published, err := s.PublishPaper(ctx, u, draft.ID, draft.Version)
		if err != nil {
			return false, err
		}
		pid = published.ID
	} else if e != nil {
		return false, e
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return false, e
	}
	defer tx.Rollback()
	if e = validTeacher(ctx, tx, u); e != nil {
		return false, e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE curriculum_seed_items SET resources_json=JSON_SET(resources_json,'$.homework_paper_id',?) WHERE class_id=? AND created_by=? AND batch=? AND unit_key=?`, pid, u.ClassID, u.ID, JuniorBatch, unit.Key); e != nil {
		return false, e
	}
	return false, tx.Commit()
}
