package tutor

import (
	"campusclaw/backend/internal/auth"
	"net/http"
	"sync"
)

func (s *Service) agentGate(id int64) *sync.RWMutex {
	v, _ := s.agentGates.LoadOrStore(id, &sync.RWMutex{})
	return v.(*sync.RWMutex)
}

// @api {"method":"DELETE","path":"/api/agents/{id}","summary":"删除本班智能体，保留只读对话历史","auth":"teacher","status":204,"response":null,"errors":{"401":"未登录","403":"仅教师可操作","404":"资源不存在","409":"该智能体仍有对话正在生成，请稍后再删除","503":"服务暂时不可用"},"description":"确认后软删除并解除技能/资料绑定，配置和可选列表移除。原学生会话保留agent_deleted=true，只能阅读/删除；允许同名重建，重启不复活种子。"}
func (s *Service) DeleteAgent(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromContext(r.Context())
	id := pathID(r)
	if _, e := s.agent(r.Context(), id, u.ClassID, false); e != nil {
		resourceError(w, e)
		return
	}
	gate := s.agentGate(id)
	if !gate.TryLock() {
		fail(w, 409, "该智能体仍有对话正在生成，请稍后再删除")
		return
	}
	defer gate.Unlock()
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		dbFail(w)
		return
	}
	defer tx.Rollback()
	var found int64
	if e = tx.QueryRowContext(r.Context(), `SELECT id FROM tutor_agents WHERE id=? AND class_id=? AND deleted_at IS NULL FOR UPDATE`, id, u.ClassID).Scan(&found); e != nil {
		resourceError(w, e)
		return
	}
	if _, e = tx.ExecContext(r.Context(), `UPDATE tutor_agents SET deleted_at=CURRENT_TIMESTAMP,enabled=FALSE WHERE id=?`, id); e != nil {
		dbFail(w)
		return
	}
	for _, table := range []string{"tutor_agent_skills", "tutor_agent_materials"} {
		if _, e = tx.ExecContext(r.Context(), `DELETE FROM `+table+` WHERE agent_id=?`, id); e != nil {
			dbFail(w)
			return
		}
	}
	if e = tx.Commit(); e != nil {
		dbFail(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @api {"method":"DELETE","path":"/api/conversations/{id}","summary":"删除本人本班对话及消息","auth":"student","status":204,"response":null,"errors":{"401":"未登录","403":"仅学生可使用智能体会话","404":"资源不存在","409":"该会话正在生成，请先停止或等待结束再删除","503":"服务暂时不可用"},"description":"不可恢复。本人本班所有权由会话读取，忽略客户端声明；级联删除消息，保留教师工具审计。正在生成时409，无副作用。"}
func (s *Service) DeleteConversation(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromContext(r.Context())
	c, e := s.conversation(r.Context(), pathID(r), u)
	if e != nil {
		resourceError(w, e)
		return
	}
	if _, loaded := s.busy.LoadOrStore(c.ID, true); loaded {
		fail(w, 409, "该会话正在生成，请先停止或等待结束再删除")
		return
	}
	defer s.busy.Delete(c.ID)
	if _, e = s.conversation(r.Context(), c.ID, u); e != nil {
		resourceError(w, e)
		return
	}
	result, e := s.DB.ExecContext(r.Context(), `DELETE FROM conversations WHERE id=? AND user_id=? AND class_id=?`, c.ID, u.ID, u.ClassID)
	if e != nil {
		dbFail(w)
		return
	}
	n, e := result.RowsAffected()
	if e != nil {
		dbFail(w)
		return
	}
	if n == 0 {
		missing(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
