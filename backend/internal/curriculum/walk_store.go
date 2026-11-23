package curriculum

import (
	"campusclaw/backend/internal/auth"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
)

const walkColumns = `id,assignment_id,subject,title,question,reference_answer,stage,version,summary,improvements_json,(active_token<>'' AND lease_until>UTC_TIMESTAMP(6)),created_at,updated_at`

func scanWalk(row scanner) (Walk, error) {
	var v Walk
	var b []byte
	e := row.Scan(&v.ID, &v.AssignmentID, &v.Subject, &v.Title, &v.Question, &v.Reference, &v.Stage, &v.Version, &v.Summary, &b, &v.Active, &v.CreatedAt, &v.UpdatedAt)
	v.Turns = []WalkTurn{}
	if e == nil {
		e = json.Unmarshal(b, &v.Improvements)
	}
	return v, normalize(e)
}

const turnColumns = `id,content,action,stage_before,stage_after,status,response,summary,improvements_json,created_at,updated_at`

func scanTurn(row scanner) (WalkTurn, error) {
	var v WalkTurn
	var b []byte
	e := row.Scan(&v.ID, &v.Content, &v.Action, &v.StageBefore, &v.StageAfter, &v.Status, &v.Response, &v.Summary, &b, &v.CreatedAt, &v.UpdatedAt)
	if e == nil {
		e = json.Unmarshal(b, &v.Improvements)
	}
	return v, normalize(e)
}
func walkOwner(ctx context.Context, q queryer, u auth.User, id int64, lock bool) (Walk, error) {
	query := `SELECT ` + walkColumns + ` FROM walkthroughs WHERE id=? AND class_id=? AND created_by=?`
	if lock {
		query += ` FOR UPDATE`
	}
	return scanWalk(q.QueryRowContext(ctx, query, id, u.ClassID, u.ID))
}
func expireWalk(ctx context.Context, tx *sql.Tx, id int64) error {
	var expired bool
	if e := tx.QueryRowContext(ctx, `SELECT active_token<>'' AND lease_until<=UTC_TIMESTAMP(6) FROM walkthroughs WHERE id=?`, id).Scan(&expired); e != nil {
		return e
	}
	if !expired {
		return nil
	}
	if _, e := tx.ExecContext(ctx, `UPDATE walkthrough_turns SET status='failed',updated_at=UTC_TIMESTAMP(6) WHERE walkthrough_id=? AND status='pending'`, id); e != nil {
		return e
	}
	_, e := tx.ExecContext(ctx, `UPDATE walkthroughs SET active_token='',lease_until=NULL WHERE id=?`, id)
	return e
}
func (s *Store) CreateWalk(ctx context.Context, u auth.User, in WalkCreate) (Walk, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return Walk{}, e
	}
	defer tx.Rollback()
	var existing, assignment int64
	e = tx.QueryRowContext(ctx, `SELECT id,assignment_id FROM walkthroughs WHERE class_id=? AND created_by=? AND client_key=?`, u.ClassID, u.ID, in.ClientKey).Scan(&existing, &assignment)
	if e == nil {
		if assignment != in.AssignmentID {
			return Walk{}, ErrConflict
		}
		tx.Rollback()
		return s.Walk(ctx, u, existing)
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return Walk{}, e
	}
	var subject, title, question, reference string
	e = tx.QueryRowContext(ctx, `SELECT subject,title,question,expected_answer FROM grade_assignments WHERE id=? AND class_id=? FOR SHARE`, in.AssignmentID, u.ClassID).Scan(&subject, &title, &question, &reference)
	if e != nil {
		return Walk{}, normalize(e)
	}
	result, e := tx.ExecContext(ctx, `INSERT INTO walkthroughs(class_id,created_by,assignment_id,subject,title,question,reference_answer,client_key,summary,improvements_json) VALUES(?,?,?,?,?,?,?,?, '',JSON_ARRAY())`, u.ClassID, u.ID, in.AssignmentID, subject, title, question, reference, in.ClientKey)
	if e != nil {
		tx.Rollback()
		if errors.Is(normalize(e), ErrConflict) {
			var id, a int64
			if e = s.DB.QueryRowContext(ctx, `SELECT id,assignment_id FROM walkthroughs WHERE class_id=? AND created_by=? AND client_key=?`, u.ClassID, u.ID, in.ClientKey).Scan(&id, &a); e == nil && a == in.AssignmentID {
				return s.Walk(ctx, u, id)
			}
			return Walk{}, ErrConflict
		}
		return Walk{}, e
	}
	id, e := result.LastInsertId()
	if e != nil {
		return Walk{}, e
	}
	if e = tx.Commit(); e != nil {
		return Walk{}, e
	}
	return s.Walk(ctx, u, id)
}
func (s *Store) Walk(ctx context.Context, u auth.User, id int64) (Walk, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return Walk{}, e
	}
	defer tx.Rollback()
	if _, e = walkOwner(ctx, tx, u, id, true); e != nil {
		return Walk{}, e
	}
	if e = expireWalk(ctx, tx, id); e != nil {
		return Walk{}, e
	}
	v, e := walkOwner(ctx, tx, u, id, false)
	if e != nil {
		return v, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT `+turnColumns+` FROM walkthrough_turns WHERE walkthrough_id=? ORDER BY id`, id)
	if e != nil {
		return v, e
	}
	for rows.Next() {
		t, e := scanTurn(rows)
		if e != nil {
			rows.Close()
			return v, e
		}
		v.Turns = append(v.Turns, t)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return v, e
	}
	return v, tx.Commit()
}
func (s *Store) Walks(ctx context.Context, u auth.User, f PointFilter) ([]Walk, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT `+walkColumns+` FROM walkthroughs WHERE class_id=? AND created_by=? AND subject=? AND title LIKE ? ORDER BY id DESC LIMIT ? OFFSET ?`, u.ClassID, u.ID, f.Subject, "%"+f.Q+"%", f.Size, int64(f.Page-1)*int64(f.Size))
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Walk{}
	for rows.Next() {
		v, e := scanWalk(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) BeginWalkTurn(ctx context.Context, u auth.User, id int64, in WalkInput) (WalkRun, error) {
	var run WalkRun
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return run, e
	}
	defer tx.Rollback()
	v, e := walkOwner(ctx, tx, u, id, true)
	if e != nil {
		return run, e
	}
	if e = expireWalk(ctx, tx, id); e != nil {
		return run, e
	}
	v, e = walkOwner(ctx, tx, u, id, false)
	if e != nil {
		return run, e
	}
	run.Walk = v
	var tid int64
	var hash, status string
	e = tx.QueryRowContext(ctx, `SELECT id,input_hash,status FROM walkthrough_turns WHERE walkthrough_id=? AND client_key=?`, id, in.ClientKey).Scan(&tid, &hash, &status)
	if e == nil {
		if hash != in.Hash() {
			return run, ErrConflict
		}
		if status == "succeeded" {
			run.Turn, e = scanTurn(tx.QueryRowContext(ctx, `SELECT `+turnColumns+` FROM walkthrough_turns WHERE id=?`, tid))
			run.Replayed = true
			return run, e
		}
	} else if !errors.Is(e, sql.ErrNoRows) {
		return run, e
	}
	if v.Active || v.Version != in.ExpectedVersion {
		return run, ErrConflict
	}
	var discussed bool
	var count int
	if e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM walkthrough_turns WHERE walkthrough_id=? AND stage_after=? AND action='discuss' AND status='succeeded'),(SELECT COUNT(*) FROM walkthrough_turns WHERE walkthrough_id=?)`, id, v.Stage, id).Scan(&discussed, &count); e != nil {
		return run, e
	}
	next, e := nextWalkStage(v.Stage, in.Action, discussed)
	if e != nil {
		return run, e
	}
	if tid == 0 && count >= 50 {
		return run, ErrConflict
	}
	token := make([]byte, 24)
	if _, e = rand.Read(token); e != nil {
		return run, e
	}
	run.Token = hex.EncodeToString(token)
	if tid == 0 {
		r, e := tx.ExecContext(ctx, `INSERT INTO walkthrough_turns(walkthrough_id,class_id,created_by,client_key,input_hash,content,action,stage_before,stage_after,response,summary,improvements_json,status,run_token) VALUES(?,?,?,?,?,?,?,?,?,'','',JSON_ARRAY(),'pending',?)`, id, u.ClassID, u.ID, in.ClientKey, in.Hash(), in.Content, in.Action, v.Stage, next, run.Token)
		if e != nil {
			return run, normalize(e)
		}
		tid, e = r.LastInsertId()
		if e != nil {
			return run, e
		}
	} else {
		if _, e = tx.ExecContext(ctx, `UPDATE walkthrough_turns SET status='pending',run_token=?,updated_at=UTC_TIMESTAMP(6) WHERE id=?`, run.Token, tid); e != nil {
			return run, e
		}
	}
	if _, e = tx.ExecContext(ctx, `UPDATE walkthroughs SET active_token=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 130 SECOND) WHERE id=?`, run.Token, id); e != nil {
		return run, e
	}
	run.Turn, e = scanTurn(tx.QueryRowContext(ctx, `SELECT `+turnColumns+` FROM walkthrough_turns WHERE id=?`, tid))
	if e != nil {
		return run, e
	}
	return run, tx.Commit()
}
func (s *Store) FinishWalkTurn(ctx context.Context, u auth.User, run WalkRun, out WalkOutput) error {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	v, e := walkOwner(ctx, tx, u, run.Walk.ID, true)
	if e != nil {
		return e
	}
	var token string
	var role string
	var class int64
	if e = tx.QueryRowContext(ctx, `SELECT role,class_id FROM users WHERE id=? FOR SHARE`, u.ID).Scan(&role, &class); e != nil {
		return e
	}
	if role != "student" || class != u.ClassID {
		return ErrNotFound
	}
	if e = tx.QueryRowContext(ctx, `SELECT active_token FROM walkthroughs WHERE id=?`, v.ID).Scan(&token); e != nil {
		return e
	}
	if !v.Active || token != run.Token || v.Version != run.Walk.Version {
		return ErrConflict
	}
	b, _ := json.Marshal(out.Improvements)
	r, e := tx.ExecContext(ctx, `UPDATE walkthrough_turns SET status='succeeded',response=?,summary=?,improvements_json=?,updated_at=UTC_TIMESTAMP(6) WHERE id=? AND run_token=? AND status='pending'`, out.Response, out.Summary, string(b), run.Turn.ID, run.Token)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return ErrConflict
	}
	_, e = tx.ExecContext(ctx, `UPDATE walkthroughs SET stage=?,summary=?,improvements_json=?,version=version+1,active_token='',lease_until=NULL,updated_at=UTC_TIMESTAMP(6) WHERE id=?`, run.Turn.StageAfter, out.Summary, string(b), v.ID)
	if e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) FailWalkTurn(ctx context.Context, run WalkRun, status string) error {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `UPDATE walkthroughs SET active_token='',lease_until=NULL WHERE id=? AND active_token=?`, run.Walk.ID, run.Token); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE walkthrough_turns SET status=?,updated_at=UTC_TIMESTAMP(6) WHERE id=? AND run_token=? AND status='pending'`, status, run.Turn.ID, run.Token); e != nil {
		return e
	}
	return tx.Commit()
}
