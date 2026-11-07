package retrieval

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sort"
	"strings"

	"campusclaw/backend/internal/config"
)

// Hit 是一条可溯源的检索命中（spec 检索结果字段完整可溯源）：
// 材料标题、切片序号、字符区间、摘录（取自 MySQL chunk_text）与材料标识齐备。
type Hit struct {
	MaterialID    int64    `json:"material_id"`
	MaterialTitle string   `json:"material_title"`
	ChunkID       int64    `json:"chunk_id"`
	ChunkIndex    int      `json:"chunk_index"`
	CharStart     int      `json:"char_start"`
	CharEnd       int      `json:"char_end"`
	Excerpt       string   `json:"excerpt"`
	KeywordScore  *float64 `json:"keyword_score,omitempty"`
	VectorScore   *float32 `json:"vector_score,omitempty"`
	RRFScore      *float64 `json:"rrf_score,omitempty"`
}

// ErrVectorUnavailable 表示向量路径依赖（嵌入网关或 Qdrant）不可用——API 层据此返回 503，
// 绝不编造相似度分数（spec 检索容错与降级）。
var ErrVectorUnavailable = fmt.Errorf("向量路径不可用")

// Searcher 实现三种检索模式。班级标识只经参数传入（来源是登录会话，design.md D6）。
type Searcher struct {
	DB               *sql.DB
	Cfg              *config.Config
	Embed            *EmbeddingClient
	Qdr              *QdrantClient
	allowedMaterials []int64
}

// allowedMaterials is nil for legacy class-wide search; empty means no evidence.
func (s *Searcher) SearchBound(ctx context.Context, classID int64, query string, materials []int64, limit int) ([]Hit, error) {
	copy := *s
	copy.allowedMaterials = materials
	if len(materials) == 0 {
		return []Hit{}, nil
	}
	return copy.Search(ctx, classID, query, "hybrid", limit)
}
func (s *Searcher) materialSQL() (string, []any) {
	if s.allowedMaterials == nil {
		return "", nil
	}
	if len(s.allowedMaterials) == 0 {
		return " AND 1=0", nil
	}
	args := make([]any, 0, len(s.allowedMaterials))
	for _, id := range s.allowedMaterials {
		args = append(args, id)
	}
	return " AND kc.material_id IN (" + strings.TrimSuffix(strings.Repeat("?,", len(args)), ",") + ")", args
}

const searchLimit = 10

// Search 按模式检索本班切片。mode ∈ keyword / vector / hybrid（默认 hybrid，调用方已校验）。
func (s *Searcher) Search(ctx context.Context, classID int64, query, mode string, limit int) ([]Hit, error) {
	if limit <= 0 || limit > searchLimit {
		limit = searchLimit
	}
	switch mode {
	case "keyword":
		return s.keyword(ctx, classID, query, limit)
	case "vector":
		return s.vector(ctx, classID, query, limit)
	default: // hybrid
		return s.hybrid(ctx, classID, query, limit)
	}
}

// keyword 仅查询 MySQL ngram 全文索引，绝不调用嵌入网关或 Qdrant（spec keyword 命中原词）。
func (s *Searcher) keyword(ctx context.Context, classID int64, query string, limit int) ([]Hit, error) {
	filter, materialArgs := s.materialSQL()
	args := append([]any{query, classID, query}, materialArgs...)
	args = append(args, limit)
	rows, err := s.DB.QueryContext(ctx, `
		SELECT kc.id, kc.chunk_index, kc.char_start, kc.char_end, kc.chunk_text, m.id, m.title,
		       MATCH(kc.chunk_text) AGAINST (?) AS score
		FROM knowledge_chunks kc
		JOIN materials m ON m.id = kc.material_id
		WHERE kc.class_id = ? AND kc.index_status = 'ready'
		  AND MATCH(kc.chunk_text) AGAINST (?)`+filter+`
		ORDER BY score DESC, kc.id ASC
		LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("关键字检索失败：%w", err)
	}
	defer rows.Close()
	hits := []Hit{} // 无命中返回空数组而非 null（spec 跨班检索 200 + 空 hits）
	for rows.Next() {
		var h Hit
		var score float64
		if err := rows.Scan(&h.ChunkID, &h.ChunkIndex, &h.CharStart, &h.CharEnd, &h.Excerpt, &h.MaterialID, &h.MaterialTitle, &score); err != nil {
			return nil, err
		}
		h.KeywordScore = &score
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

// vector 问句嵌入 → Qdrant 按 class_id 过滤 + 余弦检索（阈值过滤在 Qdrant 侧）→ 主键回表取正文。
// 回表 SQL 带班级条件再核对——即使向量侧漏过滤，他班切片也不会出现在结果里（spec 向量侧漏过滤时回表兜底）。
func (s *Searcher) vector(ctx context.Context, classID int64, query string, limit int) ([]Hit, error) {
	vec, err := s.Embed.Embed(ctx, query)
	if err != nil {
		log.Printf("问句嵌入失败：%v", err)
		return nil, ErrVectorUnavailable
	}
	pts, err := s.Qdr.SearchMaterials(ctx, classID, vec, limit, s.Cfg.VectorScoreCutoff, s.allowedMaterials)
	if err != nil {
		log.Printf("向量检索失败：%v", err)
		return nil, ErrVectorUnavailable
	}
	if len(pts) == 0 {
		return []Hit{}, nil
	}
	scores := make(map[int64]float32, len(pts))
	ids := make([]any, 0, len(pts))
	for _, p := range pts {
		scores[p.ChunkID] = p.Score
		ids = append(ids, p.ChunkID)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	filter, materialArgs := s.materialSQL()
	args := append([]any{classID}, ids...)
	args = append(args, materialArgs...)
	rows, err := s.DB.QueryContext(ctx, `
		SELECT kc.id, kc.chunk_index, kc.char_start, kc.char_end, kc.chunk_text, m.id, m.title
		FROM knowledge_chunks kc
		JOIN materials m ON m.id = kc.material_id
		WHERE kc.class_id = ? AND kc.index_status = 'ready' AND kc.id IN (`+placeholders+`)`+filter, args...)
	if err != nil {
		return nil, fmt.Errorf("回表查询失败：%w", err)
	}
	defer rows.Close()
	hits := []Hit{} // 无命中返回空数组而非 null
	for rows.Next() {
		var h Hit
		if err := rows.Scan(&h.ChunkID, &h.ChunkIndex, &h.CharStart, &h.CharEnd, &h.Excerpt, &h.MaterialID, &h.MaterialTitle); err != nil {
			return nil, err
		}
		if sc, ok := scores[h.ChunkID]; ok {
			h.VectorScore = &sc
		}
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// 按相似度降序返回
	sort.Slice(hits, func(i, j int) bool {
		var a, b float32
		if hits[i].VectorScore != nil {
			a = *hits[i].VectorScore
		}
		if hits[j].VectorScore != nil {
			b = *hits[j].VectorScore
		}
		return a > b
	})
	return hits, nil
}

// hybrid 并发执行两路、各自独立过滤后按名次 RRF 融合（k=60，常数取配置）。
// 缺席的一路不贡献分数；绝不以两路原始分数直接相加排序（spec hybrid RRF 融合）。
func (s *Searcher) hybrid(ctx context.Context, classID int64, query string, limit int) ([]Hit, error) {
	type result struct {
		hits []Hit
		err  error
	}
	kwCh := make(chan result, 1)
	vcCh := make(chan result, 1)
	go func() {
		h, err := s.keyword(ctx, classID, query, searchLimit)
		kwCh <- result{h, err}
	}()
	go func() {
		h, err := s.vector(ctx, classID, query, searchLimit)
		vcCh <- result{h, err}
	}()
	kw := <-kwCh
	vc := <-vcCh
	if kw.err != nil {
		return nil, kw.err
	}
	if vc.err != nil {
		return nil, vc.err // 向量路径故障：hybrid 整体 503，不编造分数（D8）
	}

	k := float64(s.Cfg.RRFK)
	rrf := map[int64]float64{}
	merged := map[int64]*Hit{}
	// 关键字路：名次从 1 开始，分 = 1/(k+rank)
	for i, h := range kw.hits {
		c := h
		merged[h.ChunkID] = &c
		rrf[h.ChunkID] += 1 / (k + float64(i+1))
	}
	// 向量路：同上；同一切片累加两路分数
	for i, h := range vc.hits {
		if existing, ok := merged[h.ChunkID]; ok {
			existing.VectorScore = h.VectorScore
		} else {
			c := h
			merged[h.ChunkID] = &c
		}
		rrf[h.ChunkID] += 1 / (k + float64(i+1))
	}
	hits := make([]Hit, 0, len(merged))
	for id, h := range merged {
		score := rrf[id]
		h.RRFScore = &score
		hits = append(hits, *h)
	}
	sort.Slice(hits, func(i, j int) bool {
		if *hits[i].RRFScore == *hits[j].RRFScore {
			return hits[i].ChunkID < hits[j].ChunkID
		}
		return *hits[i].RRFScore > *hits[j].RRFScore
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}
