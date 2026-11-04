package tutor

import (
	"campusclaw/backend/internal/retrieval"
	"context"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var citationIndex = regexp.MustCompile(`\[([0-9]+)\]`)

func applicationKey(seed string) string {
	if !strings.HasPrefix(seed, "app-") {
		return ""
	}
	key := strings.TrimPrefix(seed, "app-")
	for _, app := range appDefinitions() {
		if app.Key == key {
			return key
		}
	}
	return ""
}
func materialSubject(title string) string {
	title = strings.ToLower(title)
	for _, v := range []struct {
		subject string
		words   []string
	}{{"语文", []string{"语文", "阅读", "写作", "作文", "古诗", "春晓", "chinese"}}, {"数学", []string{"数学", "方程", "分数", "几何", "math"}}, {"英语", []string{"英语", "english"}}, {"科学", []string{"科学", "物理", "化学", "生物", "磁铁", "science"}}} {
		for _, w := range v.words {
			if strings.Contains(title, w) {
				return v.subject
			}
		}
	}
	return ""
}
func questionSubject(q string) string {
	q = strings.ToLower(q)
	for _, v := range []struct {
		subject string
		words   []string
	}{{"科学", []string{"科学", "物理", "化学", "生物", "磁铁", "磁力", "木块", "浮力", "水蒸气", "熔化", "沸腾", "电路", "光合作用"}}, {"数学", []string{"数学", "方程", "分数", "通分", "去括号", "几何", "面积", "周长", "三角", "分母", "分子", "乘法", "除法"}}, {"英语", []string{"英语", "english", "英文", "语法", "单词"}}, {"语文", []string{"语文", "阅读理解", "作者", "段落", "作文", "古诗", "词语", "修辞"}}} {
		for _, w := range v.words {
			if strings.Contains(q, w) {
				return v.subject
			}
		}
	}
	if strings.ContainsAny(q, "=×÷") {
		return "数学"
	}
	return ""
}

// Template boilerplate must not by itself make an unrelated lecture relevant.
func contentAnchors(q string) map[string]bool {
	for _, w := range []string{"请帮我", "请给我", "帮我", "进行", "阅读分析", "写作指导", "错题复盘", "原文片段", "我的问题与尝试", "草稿片段", "写作目标", "希望改进的地方", "我目前的理解或困惑", "请给一个直观例子", "检查我的理解", "题目", "我的作答与已知错误", "请指出首个需要检查的地方和下一步思路", "不直接给最终答案", "请保留我的原意", "解释建议的理由", "请先定位线索", "再引导我归纳", "概念", "问题", "理解", "分析", "指导", "方法", "学习", "资料", "知识", "学生", "老师", "什么", "如何", "怎么", "一个", "这个", "我的", "尝试", "说明", "思路", "检查", "下一步", "阅读", "写作", "错误", "请", "进行", "注意"} {
		q = strings.ReplaceAll(q, w, " ")
	}
	anchors := map[string]bool{}
	for _, part := range strings.FieldsFunc(strings.ToLower(q), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		r := []rune(part)
		for i := 0; i+1 < len(r); i++ {
			if unicode.IsLetter(r[i]) && unicode.IsLetter(r[i+1]) {
				anchors[string(r[i:i+2])] = true
			}
		}
	}
	return anchors
}
func evidenceSubject(key, q string) string {
	switch key {
	case "reading", "writing":
		return "语文"
	case "concept", "review":
		return questionSubject(q)
	}
	return ""
}
func (s *Service) evidenceMaterials(ctx context.Context, a Agent, q string, classID int64) ([]int64, error) {
	if a.AppKey == "" {
		return a.MaterialIDs, nil
	}
	ids := []int64{}
	subject := evidenceSubject(a.AppKey, q)
	if subject == "" || len(a.MaterialIDs) == 0 {
		return ids, nil
	}
	args := []any{classID}
	for _, id := range a.MaterialIDs {
		args = append(args, id)
	}
	query := `SELECT id,title FROM materials WHERE class_id=? AND id IN (` + strings.TrimSuffix(strings.Repeat("?,", len(a.MaterialIDs)), ",") + `)`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var title string
		if err = rows.Scan(&id, &title); err != nil {
			return nil, err
		}
		if materialSubject(title) == subject {
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}
func applicableEvidence(key, q, title, excerpt string) bool {
	if key == "" {
		return true
	}
	subject := evidenceSubject(key, q)
	if subject == "" || materialSubject(title) != subject {
		return false
	}
	score := 0
	text := strings.ToLower(excerpt)
	for word := range contentAnchors(q) {
		if strings.Contains(text, word) {
			score++
			if score >= 2 {
				return true
			}
		}
	}
	return false
}
func eligibleHits(a Agent, q string, hits []retrieval.Hit) []retrieval.Hit {
	out := []retrieval.Hit{}
	for _, h := range hits {
		if applicableEvidence(a.AppKey, q, h.MaterialTitle, h.Excerpt) {
			out = append(out, h)
		}
	}
	return out
}

// Only actually cited, applicable snapshots are projected to the user. Stored audit text is untouched.
func projectReferences(a Agent, m Message, q string) Message {
	used := map[int]bool{}
	for _, v := range citationIndex.FindAllStringSubmatch(m.Content, -1) {
		n, _ := strconv.Atoi(v[1])
		used[n] = true
	}
	valid := map[int]bool{}
	out := []retrieval.Citation{}
	for _, c := range m.Citations {
		if used[c.Index] && applicableEvidence(a.AppKey, q, c.MaterialTitle, c.Excerpt) {
			out = append(out, c)
			valid[c.Index] = true
		}
	}
	m.Content = citationIndex.ReplaceAllStringFunc(m.Content, func(v string) string {
		n, _ := strconv.Atoi(v[1 : len(v)-1])
		if valid[n] {
			return v
		}
		return ""
	})
	m.Citations = out
	return m
}
func projectHistory(a Agent, history []Message) []Message {
	out := append([]Message(nil), history...)
	q := ""
	for i := range out {
		if out[i].Role == "user" {
			q = out[i].Content
		} else if out[i].Role == "assistant" {
			out[i] = projectReferences(a, out[i], q)
		}
	}
	return out
}
