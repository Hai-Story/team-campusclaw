package tutor

import "testing"

func TestParseSkill(t *testing.T) {
	s, e := ParseSkill(seedSource)
	if e != nil || !s.Enabled {
		t.Fatal(s, e)
	}
	for _, bad := range []string{"名称：a\n启用：是\n\nx", "名称：a\n名称：b\n适用条件：a\n启用：是\n\nx", "名称：a\n适用条件：a\n工具：shell\n启用：是\n\nx", "名称：a\n适用条件：a\n启用：yes\n\nx", "名称：a\n适用条件：a\n启用：是\n未知：x\n\nx"} {
		if _, e := ParseSkill(bad); e == nil {
			t.Fatal("accepted invalid skill")
		}
	}
}
func TestRoute(t *testing.T) {
	s := []Skill{{ID: 9, Enabled: true, Trigger: "Equation|去括号"}, {ID: 2, Enabled: true, Trigger: "equation，去括号"}, {ID: 1, Enabled: false, Trigger: "equation，去括号"}}
	if v := Route(s, "这道 EQUATION 如何去括号"); v == nil || v.ID != 2 {
		t.Fatal(v)
	}
	if Route(s, "背诗") != nil {
		t.Fatal("zero score selected")
	}
}
