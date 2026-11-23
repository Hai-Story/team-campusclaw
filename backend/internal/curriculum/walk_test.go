package curriculum

import "testing"

// W1/W2: invalid inputs and deterministic progression, independent of model output.
func TestWalkBoundaries(t *testing.T) {
	good := WalkInput{Content: "我的思路", Action: "discuss", ExpectedVersion: 1, ClientKey: "case-1"}
	if good.Validate() != nil {
		t.Fatal("valid")
	}
	for _, action := range []string{"skip", "completed", ""} {
		x := good
		x.Action = action
		if x.Validate() == nil {
			t.Fatal(action)
		}
	}
	for _, stage := range walkStages {
		if stage != "completed" {
			if got, e := nextWalkStage(stage, "discuss", false); e != nil || got != stage {
				t.Fatal(stage)
			}
		}
		if _, e := nextWalkStage(stage, "advance", false); e == nil {
			t.Fatal("missing discussion", stage)
		}
	}
	for i, s := range walkStages[:3] {
		got, e := nextWalkStage(s, "advance", true)
		if e != nil || got != walkStages[i+1] {
			t.Fatal(s)
		}
	}
	if got, e := nextWalkStage("check", "finish", true); e != nil || got != "completed" {
		t.Fatal(got, e)
	}
	if _, e := nextWalkStage("read", "finish", true); e == nil {
		t.Fatal("premature finish")
	}
	o := WalkOutput{"先检查条件", "已梳理条件", []string{"补充理由"}}
	if o.Validate() != nil {
		t.Fatal("output")
	}
	o.Response = "伪造[999]"
	if o.Validate() == nil {
		t.Fatal("citation")
	}
}
