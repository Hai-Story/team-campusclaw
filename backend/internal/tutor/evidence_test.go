package tutor

import (
	"campusclaw/backend/internal/retrieval"
	"strings"
	"testing"
)

func TestAppEvidenceIndependentOfSkills(t *testing.T) {
	science := retrieval.Hit{MaterialID: 1, MaterialTitle: "A班-科学课程资料.md", Excerpt: "磁铁能吸引铁钉，不能吸引木块。"}
	reading := retrieval.Hit{MaterialID: 2, MaterialTitle: "语文阅读笔记.md", Excerpt: "递伞动作和雨幕细节能够表现人物善意。"}
	math := retrieval.Hit{MaterialID: 3, MaterialTitle: "数学方程笔记.md", Excerpt: "去括号时需运用乘法分配律，括号外系数乘每一项。"}
	for _, v := range []struct {
		key, q string
		hit    retrieval.Hit
	}{{"reading", "请帮我进行阅读分析。原文片段：他把伞递给孩子，转身走进雨幕。我的问题与尝试：递伞的动作有什么作用？", reading}, {"writing", "请给我写作指导。我的草稿片段：他递伞后走进雨幕。", reading}, {"concept", "磁铁能吸木块吗？", science}, {"review", "错题：去括号时忘了乘括号内常数，怎么检查？", math}} {
		for _, sk := range []*Skill{nil, {Name: "任意技能", Description: "任意说明", Subject: ""}} {
			msg, cites := BuildMessages(Agent{AppKey: v.key}, sk, []Message{{Role: "user", Content: v.q}}, []retrieval.Hit{v.hit})
			if len(cites) != 1 || !strings.Contains(msg[len(msg)-1].Content, v.hit.Excerpt) {
				t.Fatalf("relevant lost %s %+v", v.key, cites)
			}
		}
		if v.key != "concept" {
			msg, cites := BuildMessages(Agent{AppKey: v.key}, nil, []Message{{Role: "user", Content: v.q}}, []retrieval.Hit{science})
			if len(cites) != 0 || strings.Contains(msg[len(msg)-1].Content, science.Excerpt) {
				t.Fatal("unrelated science leaked", v.key)
			}
		}
	}
	if got := eligibleHits(Agent{AppKey: "concept"}, "你好，给我学习建议", []retrieval.Hit{science}); len(got) != 0 {
		t.Fatal("unknown subject forced evidence")
	}
}
func TestHistoricalAndActuallyUsedReferences(t *testing.T) {
	citations := []retrieval.Citation{{Index: 1, MaterialID: 1, MaterialTitle: "科学课程资料", Excerpt: "磁铁能吸引铁钉"}, {Index: 2, MaterialID: 2, MaterialTitle: "语文阅读笔记", Excerpt: "递伞动作和雨幕细节表现善意"}}
	saved := Message{Role: "assistant", Content: "分析递伞和雨幕[2]。错误来源[1]。", Citations: citations}
	out := projectHistory(Agent{AppKey: "reading"}, []Message{{Role: "user", Content: "递伞动作和雨幕有什么作用？"}, saved})
	if len(out[1].Citations) != 1 || out[1].Citations[0].Index != 2 || strings.Contains(out[1].Content, "[1]") {
		t.Fatal(out)
	}
	if saved.Content != "分析递伞和雨幕[2]。错误来源[1]。" || len(saved.Citations) != 2 {
		t.Fatal("audit changed")
	}
	unused := projectReferences(Agent{}, Message{Role: "assistant", Content: "没有引用任何来源", Citations: citations}, "")
	if len(unused.Citations) != 0 {
		t.Fatal("candidates shown as citations")
	}
}
