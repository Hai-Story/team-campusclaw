// Package retrieval 实现知识库检索全链路：
// 嵌入网关客户端、Qdrant 客户端、切片索引管线、三种检索模式（keyword/vector/hybrid + RRF）、
// 问答（先检索后生成）与索引重建。
// 关键约束（design.md D1/D3-D8、spec 各 Requirement）：
//   - 向量主键 = knowledge_chunks.id；payload 仅五标识字段，不含正文。
//   - 班级标识只从登录会话传入；Qdrant 查询与回表 SQL 双侧过滤。
//   - 嵌入失败不丢原文（index_status=failed）；Qdrant/网关故障时 keyword 仍可用，vector/hybrid 503。
package retrieval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// httpClient 是网关与 Qdrant 共用的 HTTP 客户端（带超时，避免外部依赖拖死请求）。
var httpClient = &http.Client{Timeout: 60 * time.Second}

// EmbeddingClient 调用课程嵌入网关（OpenAI 兼容 /embeddings）。
// 仅在服务端调用；密钥从环境变量注入，不下发浏览器（spec 配置与密钥边界）。
type EmbeddingClient struct {
	BaseURL string
	APIKey  string
	Model   string
	Dim     int
}

type embedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// Embed 返回文本向量；响应维度与配置不符时报错（防止写入异维向量）。
func (c *EmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error) {
	body, _ := json.Marshal(embedRequest{Model: c.Model, Input: text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("嵌入网关不可达：%w", err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 20<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("嵌入网关返回 %d：%s", res.StatusCode, truncate(string(data), 200))
	}
	var out embedResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("嵌入网关响应解析失败：%w", err)
	}
	if len(out.Data) == 0 || len(out.Data[0].Embedding) != c.Dim {
		got := 0
		if len(out.Data) > 0 {
			got = len(out.Data[0].Embedding)
		}
		return nil, fmt.Errorf("向量维度不符：期望 %d，实际 %d", c.Dim, got)
	}
	return out.Data[0].Embedding, nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
