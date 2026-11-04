package retrieval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"campusclaw/backend/internal/config"
)

// Qdrant 集合名（design.md D4）：全量切片共用一个集合，以 payload class_id 过滤实现班级隔离。
const CollectionName = "campusclaw_chunks"

// QdrantClient 访问 Qdrant REST API。仅服务端调用，Compose 内部网络，不映射宿主端口。
type QdrantClient struct {
	BaseURL string
	Dim     int
	once    sync.Once
	onceErr error
}

func NewQdrantClient(cfg *config.Config) *QdrantClient {
	return &QdrantClient{BaseURL: cfg.QdrantURL, Dim: cfg.EmbeddingDim}
}

func (c *QdrantClient) do(ctx context.Context, method, path string, payload any) (int, []byte, error) {
	var body io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("Qdrant 不可达：%w", err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 20<<20))
	if err != nil {
		return 0, nil, err
	}
	return res.StatusCode, data, nil
}

// EnsureCollection 幂等创建集合（余弦，维度取 EMBEDDING_DIM）。
// 只在进程内尝试一次（sync.Once）；失败不阻断启动——Qdrant 故障时 keyword 模式仍可用（D8）。
func (c *QdrantClient) EnsureCollection(ctx context.Context) error {
	c.once.Do(func() {
		status, _, err := c.do(ctx, http.MethodGet, "/collections/"+CollectionName, nil)
		if err != nil {
			c.onceErr = err
			return
		}
		if status == http.StatusOK {
			return
		}
		status, data, err := c.do(ctx, http.MethodPut, "/collections/"+CollectionName, map[string]any{
			"vectors": map[string]any{"size": c.Dim, "distance": "Cosine"},
		})
		if err != nil {
			c.onceErr = err
			return
		}
		if status != http.StatusOK && status != http.StatusCreated {
			c.onceErr = fmt.Errorf("创建集合失败 %d：%s", status, truncate(string(data), 200))
		}
	})
	return c.onceErr
}

// ResetEnsure 用于重建索引等场景在下一次调用前重试集合创建（不重置已成功的状态）。
func (c *QdrantClient) ResetEnsure() {
	c.once = sync.Once{}
}

// UpsertPoint 以 chunk id 为点主键写入向量；payload 仅含五个标识字段，不含正文（spec 向量库不含正文）。
func (c *QdrantClient) UpsertPoint(ctx context.Context, chunkID, classID, materialID, entryID int64, chunkIndex int, vector []float32) error {
	if err := c.EnsureCollection(ctx); err != nil {
		return err
	}
	status, data, err := c.do(ctx, http.MethodPut, "/collections/"+CollectionName+"/points?wait=true", map[string]any{
		"points": []map[string]any{{
			"id":     chunkID,
			"vector": vector,
			"payload": map[string]any{
				"class_id":           classID,
				"material_id":        materialID,
				"knowledge_entry_id": entryID,
				"chunk_id":           chunkID,
				"chunk_index":        chunkIndex,
			},
		}},
	})
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("写入向量失败 %d：%s", status, truncate(string(data), 200))
	}
	return nil
}

// DeleteByMaterial 按 material_id 过滤删除该材料的全部向量（索引重建/删除材料用）。
func (c *QdrantClient) DeleteByMaterial(ctx context.Context, materialID int64) error {
	if err := c.EnsureCollection(ctx); err != nil {
		return err
	}
	status, data, err := c.do(ctx, http.MethodPost, "/collections/"+CollectionName+"/points/delete?wait=true", map[string]any{
		"filter": map[string]any{
			"must": []map[string]any{{"key": "material_id", "match": map[string]any{"value": materialID}}},
		},
	})
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("删除向量失败 %d：%s", status, truncate(string(data), 200))
	}
	return nil
}

// ScoredPoint 是向量检索的一条候选（chunk id 与余弦相似度）。
type ScoredPoint struct {
	ChunkID int64   `json:"chunk_id"`
	Score   float32 `json:"score"`
}

// Search 在集合内按 class_id 过滤做余弦检索；低于阈值的候选被丢弃（spec 三种检索模式）。
// 回表核对班级由调用方在 MySQL 侧完成（双侧过滤 + 回表兜底）。
func (c *QdrantClient) Search(ctx context.Context, classID int64, vector []float32, limit int, scoreCutoff float64) ([]ScoredPoint, error) {
	return c.SearchMaterials(ctx, classID, vector, limit, scoreCutoff, nil)
}
func (c *QdrantClient) SearchMaterials(ctx context.Context, classID int64, vector []float32, limit int, scoreCutoff float64, materials []int64) ([]ScoredPoint, error) {
	if materials != nil && len(materials) == 0 {
		return []ScoredPoint{}, nil
	}
	must := []map[string]any{{"key": "class_id", "match": map[string]any{"value": classID}}}
	if materials != nil {
		must = append(must, map[string]any{"key": "material_id", "match": map[string]any{"any": materials}})
	}
	if err := c.EnsureCollection(ctx); err != nil {
		return nil, err
	}
	status, data, err := c.do(ctx, http.MethodPost, "/collections/"+CollectionName+"/points/search", map[string]any{
		"vector": vector,
		"limit":  limit,
		"filter": map[string]any{
			"must": must,
		},
		"score_threshold": scoreCutoff,
		"with_payload":    true,
	})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("向量检索失败 %d：%s", status, truncate(string(data), 200))
	}
	var out struct {
		Result []struct {
			ID      int64   `json:"id"`
			Score   float32 `json:"score"`
			Payload struct {
				ChunkID int64 `json:"chunk_id"`
			} `json:"payload"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("向量检索响应解析失败：%w", err)
	}
	pts := make([]ScoredPoint, 0, len(out.Result))
	for _, r := range out.Result {
		id := r.Payload.ChunkID
		if id == 0 {
			id = r.ID
		}
		pts = append(pts, ScoredPoint{ChunkID: id, Score: r.Score})
	}
	return pts, nil
}
