// Package auditlog stores bounded operation metadata without request or teaching content.
package auditlog

import (
	"context"
	"database/sql"
	"encoding/json"
	"regexp"
	"sync"
	"time"
)

type Store struct{ DB *sql.DB }
type Event struct {
	ID         int64          `json:"id"`
	RequestID  string         `json:"request_id"`
	ActorID    int64          `json:"actor_id"`
	ClassID    int64          `json:"class_id"`
	Time       time.Time      `json:"time"`
	Action     string         `json:"action"`
	Object     string         `json:"object"`
	Result     string         `json:"result"`
	HTTPStatus int            `json:"http_status"`
	SkillID    any            `json:"skill_id"`
	DurationMS int64          `json:"duration_ms"`
	Model      map[string]any `json:"model"`
	ToolStates map[string]int `json:"tool_states"`
}

var identifier = regexp.MustCompile(`^[a-zA-Z0-9_.:/ {}-]{1,200}$`)

func Safe(value string) string {
	if identifier.MatchString(value) {
		return value
	}
	return "unknown"
}
func Migrate(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS operation_audit_events (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, event_key VARCHAR(100) NOT NULL UNIQUE,
 class_id BIGINT NOT NULL DEFAULT 0, actor_id BIGINT NOT NULL DEFAULT 0,
 created_at DATETIME(6) NOT NULL, skill_id VARCHAR(80) NOT NULL DEFAULT '',
 action VARCHAR(200) NOT NULL, result VARCHAR(32) NOT NULL, payload JSON NOT NULL,
 INDEX audit_class_time(class_id,created_at,id),
 INDEX audit_actor_time(class_id,actor_id,created_at,id),
 INDEX audit_skill_time(class_id,skill_id,created_at,id)
 ) ENGINE=InnoDB`)
	return err
}
func (s *Store) Start(ctx context.Context, key string, e Event) (*Trace, error) {
	e.Time = time.Now().UTC()
	e.Result = "started"
	e.Action = Safe(e.Action)
	e.Object = Safe(e.Object)
	e.ToolStates = map[string]int{}
	raw, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	skill := ""
	if v, ok := e.SkillID.(string); ok {
		skill = Safe(v)
	}
	result, err := s.DB.ExecContext(ctx, `INSERT INTO operation_audit_events(event_key,class_id,actor_id,created_at,skill_id,action,result,payload) VALUES(?,?,?,?,?,?,'started',?) ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id)`, key, e.ClassID, e.ActorID, e.Time, skill, e.Action, raw)
	if err != nil {
		return nil, err
	}
	e.ID, err = result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &Trace{store: s, event: e}, nil
}

type traceKey struct{}
type Trace struct {
	mu    sync.Mutex
	store *Store
	event Event
}

func With(ctx context.Context, t *Trace) context.Context {
	return context.WithValue(ctx, traceKey{}, t)
}
func From(ctx context.Context) *Trace { t, _ := ctx.Value(traceKey{}).(*Trace); return t }
func Bind(ctx context.Context, actor, class int64) error {
	t := From(ctx)
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.event.ActorID = actor
	t.event.ClassID = class
	call, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return t.save(call)
}
func Tool(ctx context.Context, status string) {
	if t := From(ctx); t != nil {
		t.mu.Lock()
		defer t.mu.Unlock()
		switch status {
		case "ok", "denied", "error":
			t.event.ToolStates[status]++
		default:
			t.event.ToolStates["error"]++
		}
	}
}
func (t *Trace) save(ctx context.Context) error {
	raw, err := json.Marshal(t.event)
	if err != nil {
		return err
	}
	skill := ""
	if t.event.SkillID != nil {
		if value, ok := t.event.SkillID.(string); ok && len(value) <= 80 {
			skill = value
		} else {
			v, _ := json.Marshal(t.event.SkillID)
			if len(v) <= 80 {
				skill = string(v)
			}
		}
	}
	_, err = t.store.DB.ExecContext(ctx, `UPDATE operation_audit_events SET class_id=?,actor_id=?,skill_id=?,action=?,result=?,payload=? WHERE id=?`, t.event.ClassID, t.event.ActorID, skill, t.event.Action, t.event.Result, raw, t.event.ID)
	return err
}
func (t *Trace) Finish(ctx context.Context, action, object, result string, status int, duration int64, model map[string]any, skill any) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.event.Action = Safe(action)
	t.event.Object = Safe(object)
	if result == "succeeded" && t.event.ToolStates["denied"] > 0 {
		result = "denied"
	}
	t.event.Result = Safe(result)
	t.event.HTTPStatus = status
	t.event.DurationMS = duration
	t.event.Model = model
	t.event.SkillID = skill
	return t.save(ctx)
}

func (t *Trace) ID() int64 { t.mu.Lock(); defer t.mu.Unlock(); return t.event.ID }
