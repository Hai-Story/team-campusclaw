// Package chunking 实现三种切片策略（design.md D2 / spec「切分策略」）。
//   - auto（默认）：最大 800 字、重叠 80 字，优先在空行、换行、句号处断开；请求中的自定义参数不生效。
//   - custom：最大 100–2000 字、重叠 0–50%；可选预处理（移除 URL/邮箱、折叠连续空白）。
//   - hierarchy：按 Markdown #/##/### 分章，标题保留在该章切片内，超长章节回退 auto 规则。
//
// 偏移量按 Unicode 字符（rune）计，相对（预处理后的）输入文本；预处理不改写 knowledge_entries.body_text。
package chunking

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Chunk 是一个切片：正文、序号与相对输入文本的字符区间（rune 偏移，左闭右开）。
type Chunk struct {
	Index int
	Text  string
	Start int
	End   int
}

// Options 是 custom 策略的参数（auto/hierarchy 不使用）。
type Options struct {
	MaxChars           int  // 最大长度，100–2000
	OverlapRatio       int  // 重叠比例百分比，0–50
	RemoveURLEmail     bool // 预处理：移除 URL 与邮箱
	CollapseWhitespace bool // 预处理：折叠连续空白
}

const (
	StrategyAuto      = "auto"
	StrategyCustom    = "custom"
	StrategyHierarchy = "hierarchy"
)

var urlEmailRe = regexp.MustCompile(`https?://\S+|[\w.+-]+@[\w-]+\.[\w.]+`)
var wsRe = regexp.MustCompile(`\s+`)

// Preprocess 应用可选预处理，返回预处理后的文本（不改写调用方的原文）。
func Preprocess(text string, opts Options) string {
	out := text
	if opts.RemoveURLEmail {
		out = urlEmailRe.ReplaceAllString(out, " ")
	}
	if opts.CollapseWhitespace {
		out = wsRe.ReplaceAllString(out, " ")
	}
	return out
}

// Split 按策略切分文本。strategy 非法或 custom 参数越界时返回错误（API 层据此返回 400）。
func Split(text, strategy string, opts Options, autoMax, autoOverlap int) ([]Chunk, error) {
	switch strategy {
	case StrategyAuto:
		// spec：请求中另行填写的长度与预处理参数对 auto MUST NOT 生效——固定用服务端默认值。
		return windowSplit(text, autoMax, autoOverlap), nil
	case StrategyCustom:
		if opts.MaxChars < 100 || opts.MaxChars > 2000 {
			return nil, fmt.Errorf("custom 最大长度须在 100–2000 之间，当前 %d", opts.MaxChars)
		}
		if opts.OverlapRatio < 0 || opts.OverlapRatio > 50 {
			return nil, fmt.Errorf("custom 重叠比例须在 0–50%% 之间，当前 %d%%", opts.OverlapRatio)
		}
		processed := Preprocess(text, opts)
		overlap := opts.MaxChars * opts.OverlapRatio / 100
		return windowSplit(processed, opts.MaxChars, overlap), nil
	case StrategyHierarchy:
		return hierarchySplit(text, autoMax, autoOverlap), nil
	default:
		return nil, fmt.Errorf("非法切分策略 %q（可选 auto / custom / hierarchy）", strategy)
	}
}

// windowSplit 滑动窗口切分：窗口内优先在空行、换行、句号处断开；无断点处强制截断。
// 相邻切片重叠 overlap 字。
func windowSplit(text string, maxChars, overlap int) []Chunk {
	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return nil
	}
	var chunks []Chunk
	pos := 0
	for pos < n {
		end := pos + maxChars
		if end >= n {
			end = n
		} else {
			end = bestBreak(runes, pos, end)
		}
		seg := strings.TrimSpace(string(runes[pos:end]))
		if seg != "" {
			chunks = append(chunks, Chunk{Index: len(chunks), Text: string(runes[pos:end]), Start: pos, End: end})
		}
		if end >= n {
			break
		}
		next := end - overlap
		if next <= pos { // 防死循环：重叠不得吞掉整个窗口
			next = end
		}
		pos = next
	}
	return chunks
}

// bestBreak 在 (start, end] 内从后往前找最佳断开点：优先空行，其次换行，再次句号。
// 断开点取在断字符之后（句号保留在切片内）。
func bestBreak(runes []rune, start, end int) int {
	blankLine, newline, period := -1, -1, -1
	for i := end - 1; i > start; i-- {
		switch runes[i] {
		case '\n':
			if blankLine < 0 && i+1 < end && runes[i+1] == '\n' {
				blankLine = i + 1
			}
			if newline < 0 {
				newline = i
			}
		case '。':
			if period < 0 {
				period = i
			}
		}
	}
	switch {
	case blankLine > 0:
		return blankLine + 1
	case newline > 0:
		return newline + 1
	case period > 0:
		return period + 1
	default:
		return end // 无断点：强制截断
	}
}

// headingRe 匹配 Markdown 一~三级标题行。
var headingRe = regexp.MustCompile(`(?m)^#{1,3}\s`)

// hierarchySplit 按标题分章：每个章节的切片以该章标题开头；超长章节再按 auto 窗口规则切。
func hierarchySplit(text string, autoMax, autoOverlap int) []Chunk {
	runes := []rune(text)
	n := len(runes)
	// 以行为单位找章节边界（标题行的起始 rune 位置）。
	type section struct{ start, end int }
	var sections []section
	lineStart := 0
	cur := -1
	for i, r := range runes {
		if r == '\n' || i == n-1 {
			line := string(runes[lineStart : i+1])
			if headingRe.MatchString(line) {
				if cur >= 0 {
					sections = append(sections, section{cur, lineStart})
				}
				cur = lineStart
			}
			lineStart = i + 1
		}
	}
	if cur >= 0 {
		sections = append(sections, section{cur, n})
	}
	if len(sections) == 0 {
		// 无标题：整篇按 auto 规则切
		return windowSplit(text, autoMax, autoOverlap)
	}
	var chunks []Chunk
	for _, s := range sections {
		segText := string(runes[s.start:s.end])
		if strings.TrimSpace(segText) == "" {
			continue
		}
		if s.end-s.start <= autoMax {
			chunks = append(chunks, Chunk{Index: len(chunks), Text: segText, Start: s.start, End: s.end})
			continue
		}
		// 超长章节：回退 auto 窗口规则，偏移加上章节起点
		sub := windowSplit(segText, autoMax, autoOverlap)
		for _, c := range sub {
			chunks = append(chunks, Chunk{Index: len(chunks), Text: c.Text, Start: s.start + c.Start, End: s.start + c.End})
		}
	}
	return chunks
}

// IsSpace 供外部判断（保留导出以便单元测试）。
func IsSpace(r rune) bool { return unicode.IsSpace(r) }
