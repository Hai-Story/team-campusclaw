package auditlog

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

type Filter struct {
	ClassID, ActorID, BeforeID     int64
	Skill, Action, Result, Request string
	From, To                       time.Time
	Page, Size                     int
}

func (s *Store) List(ctx context.Context, f Filter) ([]Event, int, int64, error) {
	if f.BeforeID == 0 {
		if err := s.DB.QueryRowContext(ctx, "SELECT COALESCE(MAX(id),0) FROM operation_audit_events WHERE class_id=?", f.ClassID).Scan(&f.BeforeID); err != nil {
			return nil, 0, 0, err
		}
	}
	where := "class_id=? AND created_at>=? AND created_at<=? AND id<=?"
	args := []any{f.ClassID, f.From, f.To, f.BeforeID}
	if f.ActorID > 0 {
		where += " AND actor_id=?"
		args = append(args, f.ActorID)
	}
	for _, v := range []struct{ field, value string }{{"skill_id", f.Skill}, {"action", f.Action}, {"result", f.Result}} {
		if v.value != "" {
			where += " AND " + v.field + "=?"
			args = append(args, v.value)
		}
	}
	if f.Request != "" {
		where += " AND event_key=?"
		args = append(args, "http:"+f.Request)
	}
	var count int
	if err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM operation_audit_events WHERE "+where, args...).Scan(&count); err != nil {
		return nil, 0, 0, err
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT id,payload FROM operation_audit_events WHERE "+where+" ORDER BY id DESC LIMIT ? OFFSET ?", append(args, f.Size, (f.Page-1)*f.Size)...)
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var id int64
		var raw []byte
		if err = rows.Scan(&id, &raw); err != nil {
			return nil, 0, 0, err
		}
		var e Event
		if err = json.Unmarshal(raw, &e); err != nil {
			return nil, 0, 0, err
		}
		e.ID = id
		out = append(out, e)
	}
	return out, count, f.BeforeID, rows.Err()
}
func (s *Store) Get(ctx context.Context, class, id int64) (Event, error) {
	var e Event
	var raw []byte
	err := s.DB.QueryRowContext(ctx, `SELECT payload FROM operation_audit_events WHERE class_id=? AND id=?`, class, id).Scan(&raw)
	if err == nil {
		err = json.Unmarshal(raw, &e)
		e.ID = id
	}
	return e, err
}
func NotFound(err error) bool { return err == sql.ErrNoRows }
func Object(rpath func(string) string) string {
	for _, k := range []string{"id", "task", "message", "tool"} {
		v := rpath(k)
		if v != "" && len(v) <= 64 && strings.IndexFunc(v, func(c rune) bool { return !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') }) < 0 {
			return k + ":" + v
		}
	}
	return "collection"
}
