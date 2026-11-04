package retrieval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"campusclaw/backend/internal/config"
)

// NoHitAnswer 是无命中时的固定文案——此时 MUST NOT 调用对话网关（spec 无依据时不调用生成模型）。
const NoHitAnswer = "资料中未找到相关内容"

// ChatMessage 是对话网关消息。客户端注入的 system 消息一律丢弃（spec 客户端 system 注入被丢弃）。
type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// Citation 是问答引用的一条出处，[n] 标注与本列表顺序一致。
type Citation struct {
	Index         int    `json:"index"`
	MaterialID    int64  `json:"material_id"`
	MaterialTitle string `json:"material_title"`
	ChunkIndex    int    `json:"chunk_index"`
	CharStart     int    `json:"char_start"`
	CharEnd       int    `json:"char_end"`
	Excerpt       string `json:"excerpt"`
}

// AskResponse 是 POST /api/ask 的响应体。
type AskResponse struct {
	Answer    string     `json:"answer"`
	Citations []Citation `json:"citations"`
}

// ChatClient 调用对话网关（OpenAI 兼容 /chat/completions）。仅服务端调用，密钥不下发浏览器。
type ChatClient struct {
	BaseURL string
	APIKey  string
	Model   string
}

func (c *ChatClient) Complete(ctx context.Context, messages []ChatMessage) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":       c.Model,
		"messages":    messages,
		"temperature": 0.2, // 低温度：依据资料作答，减少自由发挥
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	res, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("对话网关不可达：%w", err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 20<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("对话网关返回 %d：%s", res.StatusCode, truncate(string(data), 200))
	}
	var out struct {
		Choices []struct {
			Message ChatMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("对话网关响应解析失败：%w", err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("对话网关未返回回答")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

// askSystemPrompt 由服务端写入的 system 提示（spec：仅允许依据编号资料作答）。
const askSystemPrompt = `你是中小学教研知识库的问答助手。仅允许依据用户消息中提供的编号资料作答，禁止使用资料之外的知识，不得编造出处。
要求：
1. 回答简短（一两句话）。
2. 在依据处标注 [n]，n 与资料编号一致；多个依据可并列标注。
3. 若资料不足以回答，直接说明资料中没有相关内容，不要勉强作答。`

// Asker 实现问答：先以混合模式检索本班前 N 条切片，有命中才调对话网关。
// 对话模块输入由检索结果组装，对话模块不自行访问数据库或向量库（design.md D7）。
type Asker struct {
	Searcher *Searcher
	Chat     *ChatClient
	Cfg      *config.Config
}

// Ask 返回问答响应。检索无命中时返回固定文案且不调网关；向量路径故障返回 ErrVectorUnavailable（503）。
func (a *Asker) Ask(ctx context.Context, classID int64, question string, history []ChatMessage) (*AskResponse, error) {
	hits, err := a.Searcher.hybrid(ctx, classID, question, a.Cfg.AskTopK)
	if err != nil {
		return nil, err
	}
	if len(hits) == 0 {
		return &AskResponse{Answer: NoHitAnswer, Citations: []Citation{}}, nil
	}

	// 组装资料编号块：[n] 与 citations 列表顺序一致
	var sb strings.Builder
	sb.WriteString("资料：\n")
	citations := make([]Citation, 0, len(hits))
	for i, h := range hits {
		fmt.Fprintf(&sb, "[%d] 材料《%s》切片%d：\n%s\n\n", i+1, h.MaterialTitle, h.ChunkIndex, h.Excerpt)
		citations = append(citations, Citation{
			Index: i + 1, MaterialID: h.MaterialID, MaterialTitle: h.MaterialTitle,
			ChunkIndex: h.ChunkIndex, CharStart: h.CharStart, CharEnd: h.CharEnd, Excerpt: h.Excerpt,
		})
	}
	fmt.Fprintf(&sb, "问题：%s", question)

	messages := []ChatMessage{{Role: "system", Content: askSystemPrompt}}
	// 历史消息可附带但不参与检索；客户端构造的 system 消息一律丢弃。
	for _, m := range history {
		if m.Role == "user" || m.Role == "assistant" {
			messages = append(messages, ChatMessage{Role: m.Role, Content: m.Content})
		}
	}
	messages = append(messages, ChatMessage{Role: "user", Content: sb.String()})

	answer, err := a.Chat.Complete(ctx, messages)
	if err != nil {
		return nil, err
	}
	return &AskResponse{Answer: answer, Citations: citations}, nil
}
