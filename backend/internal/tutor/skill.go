package tutor

import (
	"campusclaw/backend/internal/mcp"
	"context"
	"fmt"
	"sort"
	"strings"
)

type Skill struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	Trigger     string            `json:"trigger_text"`
	Subject     string            `json:"subject_text"`
	Description string            `json:"description"`
	Tools       []string          `json:"tools"`
	Source      string            `json:"source_text"`
	Enabled     bool              `json:"enabled"`
	Resources   map[string]string `json:"resources,omitempty"`
}

func short(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
func ParseSkill(source string) (Skill, error) {
	s := Skill{Source: source, Tools: []string{}}
	if len([]rune(source)) > 8000 {
		return s, fmt.Errorf("技能文件最多8000字")
	}
	source = strings.ReplaceAll(source, "\r\n", "\n")
	if strings.HasPrefix(strings.TrimSpace(source), "---\n") {
		return parseManifest(strings.TrimSpace(source))
	}
	parts := strings.SplitN(strings.TrimSpace(source), "\n\n", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return s, fmt.Errorf("元数据后须空一行并填写技能说明")
	}
	values := map[string]string{}
	for _, line := range strings.Split(parts[0], "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pair := strings.FieldsFunc(line, func(r rune) bool { return r == '：' || r == ':' })
		// Split only at first colon so descriptions of metadata cannot hide duplicate keys.
		pos := strings.IndexAny(line, "：:")
		if pos < 0 || len(pair) == 0 {
			return s, fmt.Errorf("元数据格式错误")
		}
		sep := 1
		if strings.HasPrefix(line[pos:], "：") {
			sep = 3
		}
		k, v := strings.TrimSpace(line[:pos]), strings.TrimSpace(line[pos+sep:])
		if k != "名称" && k != "适用条件" && k != "资料" && k != "工具" && k != "启用" {
			return s, fmt.Errorf("未知字段：%s", k)
		}
		if _, ok := values[k]; ok {
			return s, fmt.Errorf("重复字段：%s", k)
		}
		values[k] = v
	}
	for _, k := range []string{"名称", "适用条件", "启用"} {
		if values[k] == "" {
			return s, fmt.Errorf("缺少字段：%s", k)
		}
	}
	s.Name, s.Trigger, s.Subject, s.Description = values["名称"], values["适用条件"], values["资料"], strings.TrimSpace(parts[1])
	for _, x := range []struct {
		v string
		n int
	}{{s.Name, 32}, {s.Trigger, 120}, {s.Subject, 32}, {s.Description, 2000}} {
		if len([]rune(x.v)) > x.n {
			return s, fmt.Errorf("技能字段超过长度上限")
		}
	}
	switch values["启用"] {
	case "是":
		s.Enabled = true
	case "否":
		s.Enabled = false
	default:
		return s, fmt.Errorf("启用只能填写是或否")
	}
	for _, tool := range strings.FieldsFunc(values["工具"], func(r rune) bool { return strings.ContainsRune(",，、;；|", r) }) {
		tool = strings.TrimSpace(tool)
		if tool != "knowledge_search" {
			if _, _, err := mcp.ParseRef(tool); err != nil {
				return s, err
			}
		}
		if len(s.Tools) >= 16 {
			return s, fmt.Errorf("每个技能最多绑定16个工具")
		}
		s.Tools = append(s.Tools, tool)
	}
	return s, nil
}
func splitWords(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return strings.ContainsRune(",，、;；|", r) })
}
func Route(skills []Skill, q string) *Skill {
	skills = append([]Skill(nil), skills...)
	sort.Slice(skills, func(i, j int) bool { return skills[i].ID < skills[j].ID })
	var chosen *Skill
	best := 0
	q = strings.ToLower(q)
	for _, s := range skills {
		if !s.Enabled {
			continue
		}
		score := 0
		for _, word := range splitWords(s.Trigger) {
			if word = strings.TrimSpace(word); word != "" && strings.Contains(q, word) {
				score++
			}
		}
		if score > best {
			copy := s
			chosen = &copy
			best = score
		}
	}
	return chosen
}

type resourceKey struct{}

func withResources(ctx context.Context, r map[string]string) context.Context {
	return context.WithValue(ctx, resourceKey{}, r)
}
