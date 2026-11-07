package tutor

import (
	"campusclaw/backend/internal/retrieval"
	"strings"
	"testing"
)

func TestPromptHistoryAndEvidence(t *testing.T) {
	h := []Message{}
	for i := 0; i < 9; i++ {
		h = append(h, Message{Role: "user", Content: strings.Repeat("字", 700)})
	}
	h[8].Content = "本次问题"
	hits := []retrieval.Hit{{MaterialID: 1, MaterialTitle: "数学讲义", Excerpt: "去括号要乘每一项"}, {MaterialID: 2, MaterialTitle: "语文讲义", Excerpt: "春晓"}, {MaterialID: 1, Excerpt: ""}}
	sk := &Skill{Name: "解题引导", Subject: "数学"}
	m, c := BuildMessages(Agent{}, sk, h, hits)
	if len(m) != 7 || len(c) != 1 || c[0].Index != 1 {
		t.Fatal(len(m), c)
	}
	if strings.Contains(m[0].Content, hits[0].Excerpt) || !strings.Contains(m[0].Content, "不写最终得数") {
		t.Fatal("wrong trust boundary")
	}
	if !strings.Contains(m[len(m)-1].Content, "本次问题") || !strings.Contains(m[len(m)-1].Content, "[1]") {
		t.Fatal("missing evidence")
	}
	m, c = BuildMessages(Agent{}, nil, h, nil)
	if len(c) != 0 || !strings.Contains(m[0].Content, "不得编造材料") {
		t.Fatal(m, c)
	}
}
func TestCitationFilter(t *testing.T) {
	for _, max := range []int{0, 1} {
		f := CitationFilter{max: max}
		v := f.Feed("思路[", false) + f.Feed("1]，错误[9", false) + f.Feed("]。", false) + f.Feed("", true)
		want := "思路，错误。"
		if max == 1 {
			want = "思路[1]，错误。"
		}
		if v != want {
			t.Fatal(v)
		}
	}
}

func TestNestedCitationFilter(t *testing.T){f:=CitationFilter{max:0};if v:=f.Feed("嵌套[[",false)+f.Feed("1]]正文",true);v!="嵌套[]正文"{t.Fatal(v)}}
