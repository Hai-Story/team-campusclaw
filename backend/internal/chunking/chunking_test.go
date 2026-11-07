package chunking

import (
	"strings"
	"testing"
)

func runeLen(s string) int { return len([]rune(s)) }

// auto：>800 字文本切出的每片 ≤800 字，相邻切片存在约 80 字重叠。
func TestAutoWindowAndOverlap(t *testing.T) {
	text := strings.Repeat("甲、乙、丙、丁、戊、己、庚、辛、壬、癸。", 100) // 1000 字
	chunks, err := Split(text, StrategyAuto, Options{}, 800, 80)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 2 {
		t.Fatalf("1000 字应切出至少 2 片，实际 %d", len(chunks))
	}
	for _, c := range chunks {
		if runeLen(c.Text) > 800 {
			t.Fatalf("切片 %d 超过 800 字：%d", c.Index, runeLen(c.Text))
		}
	}
	// 相邻切片区间应有重叠（约 80 字，允许断点调整造成的偏差）。
	overlap := chunks[0].End - chunks[1].Start
	if overlap < 40 || overlap > 120 {
		t.Fatalf("相邻切片重叠应约 80 字，实际 %d", overlap)
	}
	// auto 请求中的自定义参数 MUST NOT 生效。
	chunks2, _ := Split(text, StrategyAuto, Options{MaxChars: 50, RemoveURLEmail: true}, 800, 80)
	if len(chunks2) != len(chunks) {
		t.Fatal("auto 策略不应响应 custom 参数")
	}
}

// custom：参数越界报错（API 层据此 400）；预处理生效且原文不受影响。
func TestCustomValidationAndPreprocess(t *testing.T) {
	text := "访问 https://example.com/x 获取，邮箱 a@b.com。  多余  空白。"
	if _, err := Split(text, StrategyCustom, Options{MaxChars: 50}, 800, 80); err == nil {
		t.Fatal("MaxChars=50 应报错")
	}
	if _, err := Split(text, StrategyCustom, Options{MaxChars: 3000}, 800, 80); err == nil {
		t.Fatal("MaxChars=3000 应报错")
	}
	if _, err := Split(text, StrategyCustom, Options{MaxChars: 200, OverlapRatio: 60}, 800, 80); err == nil {
		t.Fatal("OverlapRatio=60 应报错")
	}
	chunks, err := Split(text, StrategyCustom, Options{MaxChars: 200, RemoveURLEmail: true, CollapseWhitespace: true}, 800, 80)
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, c := range chunks {
		joined += c.Text
	}
	if strings.Contains(joined, "https://") || strings.Contains(joined, "@") {
		t.Fatal("URL/邮箱预处理未生效")
	}
	if strings.Contains(joined, "  ") {
		t.Fatal("空白折叠预处理未生效")
	}
}

// hierarchy：带标题 Markdown 的首切片以对应标题开头；超长章节回退 auto。
func TestHierarchySections(t *testing.T) {
	text := "# 第一章 磁铁\n" + strings.Repeat("磁铁能吸引铁钉。", 150) + "\n## 第二节 水\n水在零摄氏度时结冰。"
	chunks, err := Split(text, StrategyHierarchy, Options{}, 800, 80)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 2 {
		t.Fatalf("应至少切出 2 片，实际 %d", len(chunks))
	}
	if !strings.HasPrefix(chunks[0].Text, "# 第一章 磁铁") {
		t.Fatalf("首切片应以章标题开头，实际：%q", chunks[0].Text[:20])
	}
	last := chunks[len(chunks)-1]
	if !strings.HasPrefix(last.Text, "## 第二节 水") {
		t.Fatalf("末节切片应以节标题开头，实际：%q", last.Text[:20])
	}
	// 超长章节的子切片不超过 800 字。
	for _, c := range chunks {
		if runeLen(c.Text) > 800 {
			t.Fatalf("超长章节子切片超过 800 字：%d", runeLen(c.Text))
		}
	}
}

// 无标题文本的 hierarchy 退化为 auto 窗口切分。
func TestHierarchyNoHeadings(t *testing.T) {
	text := strings.Repeat("没有标题的正文。", 100)
	chunks, err := Split(text, StrategyHierarchy, Options{}, 800, 80)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) == 0 {
		t.Fatal("无标题也应产出切片")
	}
}
