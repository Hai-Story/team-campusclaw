package tutor

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"unicode/utf8"
)

// parseManifest supports an intentionally small, documented YAML front matter subset.
func parseManifest(source string) (Skill, error) {
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	end := -1
	values := map[string]string{}
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			end = i
			break
		}
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			return Skill{}, fmt.Errorf("SKILL.md 元数据须为单行 key: value")
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if _, ok = values[k]; ok {
			return Skill{}, fmt.Errorf("重复字段：%s", k)
		}
		switch k {
		case "name", "description", "triggers", "subject", "tools", "enabled":
		default:
			return Skill{}, fmt.Errorf("不支持的元数据字段：%s", k)
		}
		if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
			v = strings.Trim(v, "[]")
			parts := strings.Split(v, ",")
			for j := range parts {
				parts[j] = strings.Trim(strings.TrimSpace(parts[j]), "\"'")
			}
			v = strings.Join(parts, ",")
		} else if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		if v == "|" || v == ">" {
			return Skill{}, fmt.Errorf("元数据不支持多行 YAML；请使用单行值")
		}
		values[k] = v
	}
	if end < 0 || values["name"] == "" || values["description"] == "" {
		return Skill{}, fmt.Errorf("SKILL.md 须包含 --- 元数据区、name 和 description")
	}
	trigger := values["triggers"]
	if trigger == "" {
		trigger = values["name"] + "," + values["description"]
	}
	enabled := "是"
	switch values["enabled"] {
	case "", "true", "是":
	case "false", "否":
		enabled = "否"
	default:
		return Skill{}, fmt.Errorf("enabled 只能为 true 或 false")
	}
	legacy := fmt.Sprintf("名称：%s\n适用条件：%s\n资料：%s\n工具：%s\n启用：%s\n\n%s", values["name"], trigger, values["subject"], values["tools"], enabled, strings.Join(lines[end+1:], "\n"))
	sk, err := ParseSkill(legacy)
	sk.Source = source
	return sk, err
}

func readSkillUpload(filename string, data []byte) (string, map[string]string, error) {
	resources := map[string]string{}
	if strings.EqualFold(path.Ext(filename), ".zip") {
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return "", nil, fmt.Errorf("无效 ZIP 文件")
		}
		if len(zr.File) > 16 {
			return "", nil, fmt.Errorf("技能包最多16项")
		}
		seen := map[string]bool{}
		manifest := ""
		base := ""
		total := 0
		for _, f := range zr.File {
			n := f.Name
			if n == "" || strings.Contains(n, "\\") || strings.HasPrefix(n, "/") || strings.Contains(n, ":") || path.Clean(n) != strings.TrimSuffix(n, "/") || f.Mode()&os.ModeType != 0 && !f.FileInfo().IsDir() {
				return "", nil, fmt.Errorf("技能包含不安全路径或链接")
			}
			for _, part := range strings.Split(n, "/") {
				if part == ".." {
					return "", nil, fmt.Errorf("技能包路径越界")
				}
			}
			if seen[n] {
				return "", nil, fmt.Errorf("技能包文件重复")
			}
			seen[n] = true
			if f.FileInfo().IsDir() {
				continue
			}
			if path.Base(n) == "SKILL.md" {
				if manifest != "" {
					return "", nil, fmt.Errorf("只能包含一个 SKILL.md")
				}
				manifest = n
				base = strings.TrimSuffix(n, "SKILL.md")
				if strings.Count(base, "/") > 1 {
					return "", nil, fmt.Errorf("SKILL.md 只允许在根目录或单一包目录")
				}
			}
		}
		if manifest == "" {
			return "", nil, fmt.Errorf("技能包必须包含 SKILL.md")
		}
		source := ""
		for _, f := range zr.File {
			if f.FileInfo().IsDir() {
				continue
			}
			n := strings.TrimPrefix(f.Name, base)
			if f.Name != manifest && (!strings.HasPrefix(f.Name, base) || !(strings.HasPrefix(n, "resources/") || strings.HasPrefix(n, "references/"))) {
				return "", nil, fmt.Errorf("仅允许 SKILL.md 和 resources/references 文本附件")
			}
			ext := strings.ToLower(path.Ext(n))
			if ext != ".md" && ext != ".txt" {
				return "", nil, fmt.Errorf("技能附件只支持 .md/.txt，不执行脚本")
			}
			if f.UncompressedSize64 > 32<<10 {
				return "", nil, fmt.Errorf("单文件最多32KiB")
			}
			r, err := f.Open()
			if err != nil {
				return "", nil, fmt.Errorf("技能包读取失败")
			}
			b, err := io.ReadAll(io.LimitReader(r, (32<<10)+1))
			r.Close()
			total += len(b)
			if err != nil || len(b) > 32<<10 || total > 128<<10 {
				return "", nil, fmt.Errorf("技能包解压超限")
			}
			if !utf8.Valid(b) || bytes.ContainsRune(b, 0) {
				return "", nil, fmt.Errorf("技能文件必须为 UTF-8 文本")
			}
			if f.Name == manifest {
				source = string(b)
			} else {
				resources[n] = string(b)
			}
		}
		if !strings.HasPrefix(strings.TrimSpace(source), "---\n") && !strings.HasPrefix(strings.TrimSpace(source), "---\r\n") {
			return "", nil, fmt.Errorf("ZIP 中 SKILL.md 须使用标准元数据格式")
		}
		return source, resources, nil
	}
	ext := strings.ToLower(path.Ext(filename))
	if ext != ".md" && ext != ".txt" {
		return "", nil, fmt.Errorf("请上传 SKILL.md、.md/.txt 或包含 SKILL.md 的 ZIP")
	}
	if len(data) > 32<<10 || !utf8.Valid(data) || bytes.ContainsRune(data, 0) {
		return "", nil, fmt.Errorf("技能文件须为32KiB以内 UTF-8 文本")
	}
	return string(data), resources, nil
}

// @api {"method":"POST","path":"/api/skills/import","summary":"上传 SKILL.md、兼容文本或含 SKILL.md 的 ZIP 技能包","auth":"teacher","status":201,"requestMedia":"multipart/form-data","request":{"type":"object","required":["file"],"properties":{"file":{"type":"string","format":"binary"}}},"response":{"$ref":"#/components/schemas/Skill"},"errors":{"400":"格式、元数据、工具引用或包边界非法","401":"未登录","403":"仅教师可操作","503":"服务暂时不可用"},"description":"ZIP 最多16项，解压总量128KiB，单文件32KiB；只含一个SKILL.md及resources/references下的UTF-8 .md/.txt。不执行脚本，不自动注入全部附件。"}
func (s *Service) ImportSkill(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	if err := r.ParseMultipartForm(256 << 10); err != nil {
		fail(w, 400, "技能包不得超过256KiB")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	f, h, err := r.FormFile("file")
	if err != nil {
		fail(w, 400, "请选择技能文件")
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (256<<10)+1))
	if err != nil || len(data) > 256<<10 {
		fail(w, 400, "技能文件超限")
		return
	}
	source, resources, err := readSkillUpload(h.Filename, data)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	ctx := withResources(r.Context(), resources)
	b, _ := json.Marshal(map[string]string{"source_text": source})
	r = r.WithContext(ctx)
	r.Body = io.NopCloser(bytes.NewReader(b))
	s.SaveSkill(w, r)
}
