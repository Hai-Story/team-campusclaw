package mcp

import (
	"campusclaw/backend/internal/auth"
	"context"
)

// registerLearning preserves any pre-existing external server with the reserved name.
func (m *Manager) registerLearning(ctx context.Context, u auth.User) (Server, error) {
	if m.Learning == nil {
		return Server{}, &apiError{503, "学习工具暂时不可用，请稍后重试"}
	}
	tx, e := m.DB.BeginTx(ctx, nil)
	if e != nil {
		return Server{}, e
	}
	defer tx.Rollback()
	var id int64
	if e = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=? AND class_id=? AND role='teacher' FOR SHARE`, u.ID, u.ClassID).Scan(&id); e != nil {
		return Server{}, e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO mcp_servers(class_id,name,base_url,token_cipher) VALUES(?,'learning',?,'') ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id)`, u.ClassID, LearningURL); e != nil {
		return Server{}, e
	}
	var url string
	if e = tx.QueryRowContext(ctx, `SELECT id,base_url FROM mcp_servers WHERE class_id=? AND name='learning' FOR UPDATE`, u.ClassID).Scan(&id, &url); e != nil {
		return Server{}, e
	}
	if url != LearningURL {
		return Server{}, &apiError{409, "learning名称已被其他服务使用，请先调整名称"}
	}
	if e = tx.Commit(); e != nil {
		return Server{}, e
	}
	return m.Get(ctx, u.ClassID, id)
}
func (m *Manager) learningServer(ctx context.Context, class int64) (Server, error) {
	var id int64
	e := m.DB.QueryRowContext(ctx, `SELECT id FROM mcp_servers WHERE class_id=? AND name='learning' AND base_url=? AND connected=TRUE`, class, LearningURL).Scan(&id)
	if e != nil {
		return Server{}, e
	}
	return m.Get(ctx, class, id)
}
