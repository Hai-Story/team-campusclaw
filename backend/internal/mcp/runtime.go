package mcp

import (
	"campusclaw/backend/internal/auth"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Candidate struct {
	ServerID   int64
	ServerName string
	Ref        string
	Alias      string
	Tool       Tool
}

// Prepare is limited to the matched skill. Unavailable bindings become explicit denied events.
func (m *Manager) Prepare(ctx context.Context, u auth.User, conversation, requestID int64, refs []string) ([]Candidate, []Call, error) {
	out := []Candidate{}
	denied := []Call{}
	seen := map[string]bool{}
	for _, ref := range refs {
		if ref == "knowledge_search" || seen[ref] {
			continue
		}
		seen[ref] = true
		name, server, e := ParseRef(ref)
		if e != nil {
			denied = append(denied, Call{Ref: ref, Status: "denied", Text: "工具引用无效"})
			continue
		}
		var id int64
		e = m.DB.QueryRowContext(ctx, `SELECT id FROM mcp_servers WHERE class_id=? AND name=?`, u.ClassID, server).Scan(&id)
		if e == sql.ErrNoRows {
			denied = append(denied, Call{Ref: ref, Status: "denied", Text: "服务器不存在或不在本班"})
			continue
		}
		if e != nil {
			return out, denied, e
		}
		s, e := m.Get(ctx, u.ClassID, id)
		if e != nil {
			return out, denied, e
		}
		candidate := Candidate{ServerID: id, ServerName: server, Ref: ref, Tool: Tool{Name: name}, Alias: fmt.Sprintf("mcp_%d_%s", id, name)}
		reason := "服务器未连接"
		if s.Connected {
			ts, e := m.tools(ctx, id)
			if e != nil {
				return out, denied, e
			}
			reason = "工具未发现、未审阅或不允许执行"
			for _, t := range ts {
				if t.Name == name && t.Approved && readOnly(t) {
					candidate.Tool = t
					reason = ""
					break
				}
			}
		}
		if reason != "" {
			c, e := m.Record(ctx, u, conversation, requestID, candidate, 0, "denied", reason)
			denied = append(denied, c)
			if e != nil {
				return out, denied, e
			}
		} else {
			out = append(out, candidate)
		}
	}
	return out, denied, nil
}
func (m *Manager) Used(ctx context.Context, conversation int64) (int, error) {
	var n int
	e := m.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM mcp_tool_calls WHERE conversation_id=?`, conversation).Scan(&n)
	return n, e
}
func (m *Manager) Record(ctx context.Context, u auth.User, conversation, requestID int64, c Candidate, round int, status, text string) (Call, error) {
	result := Call{Ref: c.Ref, Status: status, Text: text, Round: round, ServerName: c.ServerName, UserID: u.ID, ConversationID: conversation, RequestID: requestID}
	if c.ServerID == 0 {
		return result, nil
	}
	// Audit survives a cancelled request, like the streamed assistant's partial text.
	save, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	r, e := m.DB.ExecContext(save, `INSERT INTO mcp_tool_calls(class_id,server_id,server_name,tool_name,tool_ref,user_id,conversation_id,request_message_id,round_number,status,summary) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, u.ClassID, c.ServerID, c.ServerName, c.Tool.Name, c.Ref, u.ID, conversation, requestID, round, status, text)
	if e != nil {
		return result, e
	}
	result.ID, e = r.LastInsertId()
	return result, e
}
func (m *Manager) Execute(ctx context.Context, u auth.User, conversation, requestID int64, c Candidate, round int, args json.RawMessage) (Call, error) {
	l := m.lock(c.ServerID)
	l.RLock()
	defer l.RUnlock()
	status, text := "denied", "服务器不存在或不在本班"
	s, e := m.Get(ctx, u.ClassID, c.ServerID)
	if e != nil && e != sql.ErrNoRows {
		return Call{}, e
	}
	if e == nil {
		text = "服务器未连接或引用已经失效"
		if s.Connected && s.Name == c.ServerName && m.allowed[s.BaseURL] {
			ts, e := m.tools(ctx, c.ServerID)
			if e != nil {
				return Call{}, e
			}
			authorized := false
			for _, t := range ts {
				if t.Name == c.Tool.Name && t.Fingerprint == c.Tool.Fingerprint && t.Approved && readOnly(t) {
					authorized = true
					break
				}
			}
			text = "工具授权或说明已变化，请重新审阅"
			if authorized {
				if e = ValidateArguments(c.Tool.InputSchema, args); e != nil {
					text = e.Error()
				} else {
					token, err := decrypt(m.key, s.cipher)
					status = "error"
					if err != nil {
						text = err.Error()
					} else {
						var result struct {
							Content []struct {
								Type string `json:"type"`
								Text string `json:"text"`
							} `json:"content"`
							IsError bool `json:"isError"`
						}
						err = rpc(ctx, s.BaseURL, token, "tools/call", map[string]any{"name": c.Tool.Name, "arguments": args}, &result)
						if err != nil {
							text = err.Error()
						} else if result.IsError {
							text = "远端工具报告执行失败"
						} else {
							var pieces []string
							for _, p := range result.Content {
								if p.Type == "text" {
									pieces = append(pieces, p.Text)
								}
							}
							raw := strings.Join(pieces, "\n")
							if strings.TrimSpace(raw) == "" {
								text = "工具未返回文本内容"
							} else {
								var safe bool
								text, safe = SafeText(raw)
								if safe {
									status = "ok"
								}
							}
						}
					}
				}
			}
		}
	}
	if errors.Is(e, sql.ErrNoRows) {
		c.ServerID = 0
	} // Deleted/missing resources have no attributable server audit.
	return m.Record(ctx, u, conversation, requestID, c, round, status, text)
}
