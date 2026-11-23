package mcp

import (
	"campusclaw/backend/internal/auth"
	"context"
	"encoding/json"
	"time"
)

const LearningURL = "builtin://learning"

type LearningProvider interface {
	LearningTools() []Tool
	CallLearning(context.Context, auth.User, string, json.RawMessage) (string, error)
}
type nativeWindow struct {
	Start time.Time
	Count int
}

func (m *Manager) nativePermit(actor int64, now time.Time) bool {
	m.nativeMu.Lock()
	defer m.nativeMu.Unlock()
	if m.nativeLimits == nil {
		m.nativeLimits = map[int64]nativeWindow{}
	}
	v := m.nativeLimits[actor]
	if now.Sub(v.Start) >= time.Minute || now.Before(v.Start) {
		v = nativeWindow{Start: now}
	}
	if v.Count >= 30 {
		return false
	}
	v.Count++
	m.nativeLimits[actor] = v
	return true
}
func (m *Manager) currentLearningTool(t Tool) bool {
	if m.Learning == nil {
		return false
	}
	for _, current := range m.Learning.LearningTools() {
		if current.Name == t.Name && fingerprint(current) == t.Fingerprint {
			return true
		}
	}
	return false
}
func (m *Manager) executeLearning(ctx context.Context, u auth.User, conversation, request int64, c Candidate, round int, args json.RawMessage) (Call, error) {
	status, text := "denied", "工具定义已变化，请重新连接并审阅"
	if m.currentLearningTool(c.Tool) {
		var e error
		text, e = m.Learning.CallLearning(ctx, u, c.Tool.Name, args)
		status = "error"
		if e != nil {
			text = "学习数据暂时无法读取，请核对参数或稍后重试"
		} else if len([]rune(text)) > 2000 {
			text = "查询结果过长，请缩小范围"
		} else {
			var safe bool
			text, safe = SafeText(text)
			if safe {
				status = "ok"
			} else {
				status = "denied"
			}
		}
	}
	return m.Record(ctx, u, conversation, request, c, round, status, text)
}
