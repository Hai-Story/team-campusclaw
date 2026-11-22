package curriculum

import (
	"campusclaw/backend/internal/auth"
	"campusclaw/backend/internal/mcp"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

func (s *Service) LearningTools() []mcp.Tool {
	subject := map[string]any{"type": "string", "enum": []string{"chinese", "math", "english", "science", "morality"}}
	makeTool := func(name, description string, properties map[string]any) mcp.Tool {
		schema, _ := json.Marshal(map[string]any{"type": "object", "additionalProperties": false, "required": []string{"subject"}, "properties": properties})
		return mcp.Tool{Name: name, Description: description, InputSchema: schema, Annotations: map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false, "campusclawVersion": "learning-tools-v1"}}
	}
	return []mcp.Tool{
		makeTool("knowledge_points", "查询本班某学科的知识点名称、章节与本人收藏。subject必须选chinese/math/english/science/morality；q可按名称或章节查找，每页最多5项；不接收身份、班级或文件参数。", map[string]any{"subject": subject, "q": map[string]any{"type": "string", "maxLength": 100}, "page": map[string]any{"type": "integer", "minimum": 1}, "page_size": map[string]any{"type": "integer", "minimum": 1, "maximum": 5}}),
		makeTool("assignment_stats", "读取教师确认的最新作答统计：样本、得分率、最终正确率及错因。学生仅本人，教师得到本班汇总；subject为五学科，days为1至180天，默认30天。不读取AI初判或旧版本，不接收身份参数，不修改分数。", map[string]any{"subject": subject, "days": map[string]any{"type": "integer", "minimum": 1, "maximum": 180}}),
	}
}
func (s *Service) CallLearning(ctx context.Context, u auth.User, name string, raw json.RawMessage) (string, error) {
	if e := s.Store.CheckLearningActor(ctx, u); e != nil {
		return "", ErrNotFound
	}
	var value any
	switch name {
	case "knowledge_points":
		var args struct {
			Subject string `json:"subject"`
			Q       string `json:"q"`
			Page    int    `json:"page"`
			Size    int    `json:"page_size"`
		}
		args.Page = 1
		args.Size = 5
		if json.Unmarshal(raw, &args) != nil || !ValidSubject(args.Subject) || args.Page < 1 || args.Size < 1 || args.Size > 5 || len([]rune(args.Q)) > 100 {
			return "", ErrInvalid
		}
		rows, e := s.Store.Points(ctx, u, PointFilter{Subject: args.Subject, Q: strings.TrimSpace(args.Q), Page: args.Page, Size: args.Size})
		if e != nil {
			return "", errors.New("知识点暂时不可用")
		}
		type item struct {
			ID       int64  `json:"id"`
			Name     string `json:"name"`
			Chapter  string `json:"chapter"`
			Favorite bool   `json:"favorite"`
		}
		items := []item{}
		for _, p := range rows {
			items = append(items, item{p.ID, p.Name, p.Chapter, p.Favorite})
		}
		value = map[string]any{"items": items, "page": args.Page, "page_size": args.Size}
	case "assignment_stats":
		var args struct {
			Subject string `json:"subject"`
			Days    int    `json:"days"`
		}
		args.Days = 30
		if json.Unmarshal(raw, &args) != nil {
			return "", ErrInvalid
		}
		f := DiagnosisFilter{Subject: args.Subject, Days: args.Days, Page: 1, Size: 20}
		if f.Validate(time.Now()) != nil {
			return "", ErrInvalid
		}
		report, e := s.Store.Diagnosis(ctx, u, f)
		if e != nil {
			return "", errors.New("统计暂时不可用")
		}
		type item struct {
			ID             int64    `json:"id"`
			Name           string   `json:"name"`
			Samples        int      `json:"samples"`
			ScorePercent   *float64 `json:"score_percent"`
			CorrectPercent *float64 `json:"correct_percent"`
		}
		points := []item{}
		for i, p := range report.Points {
			if i == 5 {
				break
			}
			points = append(points, item{p.ID, p.Name, p.Samples, p.ScorePercent, p.CorrectPercent})
		}
		value = map[string]any{"subject": report.Subject, "from": report.From, "to": report.To, "totals": report.LearningStats, "student_count": report.StudentCount, "error_counts": report.ErrorCounts, "points_preview": points, "point_count": len(report.Points), "fingerprint": report.Fingerprint, "source": "teacher_final"}
	default:
		return "", ErrInvalid
	}
	body, e := json.Marshal(value)
	return string(body), e
}
