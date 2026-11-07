package retrieval

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"campusclaw/backend/internal/auth"
	"campusclaw/backend/internal/chunking"
	"campusclaw/backend/internal/config"
)

// Service 汇总检索链路的 HTTP 处理器。
type Service struct {
	Searcher *Searcher
	Asker    *Asker
	Indexer  *Indexer
}

func NewService(d *sql.DB, cfg *config.Config, embed *EmbeddingClient, qdr *QdrantClient, chat *ChatClient) *Service {
	searcher := &Searcher{DB: d, Cfg: cfg, Embed: embed, Qdr: qdr}
	return &Service{
		Searcher: searcher,
		Asker:    &Asker{Searcher: searcher, Chat: chat, Cfg: cfg},
		Indexer:  &Indexer{DB: d, Cfg: cfg, Embed: embed, Qdr: qdr},
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// notFound 与 materials 包保持同形：跨班与不存在完全同形（spec 索引重建 Scenario 跨班 404）。
func notFound(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "材料不存在"})
}

type searchRequest struct {
	Query string `json:"query"`
	Mode  string `json:"mode"`
}

// Search 处理 POST /api/search（会话中间件之后）。班级只取自会话；请求中的 class_id 一律丢弃。
// @api {"method":"POST","path":"/api/search","summary":"本班关键字/向量/混合检索","auth":"login","status":200,"request":{"type":"object","properties":{"query":{"type":"string","minLength":1},"mode":{"type":"string","enum":["keyword","vector","hybrid"],"default":"hybrid"}},"required":["query"]},"response":{"type":"object","properties":{"mode":{"type":"string"},"hits":{"type":"array","items":{"$ref":"#/components/schemas/Hit"}}}},"errors":{"400":"空query或非法mode","401":"未登录","503":"检索暂时不可用"}}
func (s *Service) Search(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromContext(r.Context())
	var req searchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式非法"})
		return
	}
	query := strings.TrimSpace(req.Query)
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = "hybrid" // 默认混合模式（spec 三种检索模式）
	}
	if query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query 不能为空"})
		return
	}
	if mode != "keyword" && mode != "vector" && mode != "hybrid" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "非法检索模式（可选 keyword / vector / hybrid）"})
		return
	}
	hits, err := s.Searcher.Search(r.Context(), u.ClassID, query, mode, searchLimit)
	if err != nil {
		if errors.Is(err, ErrVectorUnavailable) {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "向量检索暂时不可用"})
			return
		}
		log.Printf("检索失败：%v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "服务暂时不可用"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"mode": mode, "hits": hits})
}

type askRequest struct {
	Question string        `json:"question"`
	History  []ChatMessage `json:"history"`
}

// Ask 处理 POST /api/ask（会话中间件之后）。问答班级边界与检索一致。
// @api {"method":"POST","path":"/api/ask","summary":"单次依据问答；无命中不调用模型","auth":"login","status":200,"request":{"type":"object","properties":{"question":{"type":"string","minLength":1},"history":{"type":"array","items":{"type":"object","properties":{"role":{"type":"string"},"content":{"type":"string"}}}}},"required":["question"]},"response":{"type":"object","properties":{"answer":{"type":"string"},"citations":{"type":"array","items":{"$ref":"#/components/schemas/Citation"}}}},"errors":{"400":"空question","401":"未登录","503":"问答暂时不可用"}}
func (s *Service) Ask(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromContext(r.Context())
	var req askRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式非法"})
		return
	}
	question := strings.TrimSpace(req.Question)
	if question == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "question 不能为空"})
		return
	}
	resp, err := s.Asker.Ask(r.Context(), u.ClassID, question, req.History)
	if err != nil {
		if errors.Is(err, ErrVectorUnavailable) {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "问答暂时不可用"})
			return
		}
		log.Printf("问答失败：%v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "服务暂时不可用"})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

type reindexRequest struct {
	Strategy           string `json:"strategy"`
	MaxChars           int    `json:"max_chars"`
	OverlapRatio       int    `json:"overlap_ratio"`
	RemoveURLEmail     bool   `json:"remove_url_email"`
	CollapseWhitespace bool   `json:"collapse_whitespace"`
}

// Reindex 处理 POST /api/materials/{id}/reindex（会话 + RequireTeacher 之后）。
// 参数越界 400 且既有切片与向量不变；跨班材料与不存在同形 404。
// @api {"method":"POST","path":"/api/materials/{id}/reindex","summary":"教师重建本班材料索引","auth":"teacher","status":200,"request":{"type":"object","properties":{"strategy":{"type":"string","enum":["auto","custom","hierarchy"]},"max_chars":{"type":"integer","format":"int64"},"overlap_ratio":{"type":"number"},"remove_url_email":{"type":"boolean"},"collapse_whitespace":{"type":"boolean"}}},"response":{"type":"object","properties":{"ok":{"type":"boolean"},"chunks":{"type":"integer","format":"int64"},"strategy":{"type":"string"}}},"errors":{"400":"参数非法或本班名称重复","401":"未登录","403":"仅教师可写","404":"资源不存在（他班与不存在同形）","503":"服务暂时不可用"}}
func (s *Service) Reindex(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		notFound(w)
		return
	}
	var req reindexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式非法"})
		return
	}
	strategy := req.Strategy
	if strategy == "" {
		strategy = chunking.StrategyAuto
	}
	if strategy != chunking.StrategyAuto && strategy != chunking.StrategyCustom && strategy != chunking.StrategyHierarchy {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "非法切分策略（可选 auto / custom / hierarchy）"})
		return
	}
	// custom 参数预校验：越界 400，既有切片与向量保持不变（spec custom 参数校验）
	if strategy == chunking.StrategyCustom {
		if req.MaxChars < 100 || req.MaxChars > 2000 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "custom 最大长度须在 100–2000 之间"})
			return
		}
		if req.OverlapRatio < 0 || req.OverlapRatio > 50 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "custom 重叠比例须在 0–50% 之间"})
			return
		}
	}

	// 先取行再核对班级（与 materials 的 fetch-then-check 一致）。
	var classID, entryID int64
	var body string
	err = s.Indexer.DB.QueryRowContext(r.Context(), `
		SELECT m.class_id, e.id, e.body_text
		FROM materials m JOIN knowledge_entries e ON e.material_id = m.id
		WHERE m.id = ?`, id).Scan(&classID, &entryID, &body)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("reindex 查询失败：%v", err)
		}
		notFound(w)
		return
	}
	if classID != u.ClassID {
		log.Printf("安全事件：用户 %s（班级 %d）尝试跨班重建材料 %d（班级 %d）", u.Username, u.ClassID, id, classID)
		notFound(w)
		return
	}

	n, err := s.Indexer.IndexMaterial(r.Context(), id, entryID, classID, body, strategy, chunking.Options{
		MaxChars: req.MaxChars, OverlapRatio: req.OverlapRatio,
		RemoveURLEmail: req.RemoveURLEmail, CollapseWhitespace: req.CollapseWhitespace,
	})
	if err != nil {
		log.Printf("重建索引失败（material %d）：%v", id, err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "重建失败：" + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "chunks": n, "strategy": strategy})
}
