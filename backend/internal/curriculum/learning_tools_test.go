package curriculum

import (
	"campusclaw/backend/internal/mcp"
	"encoding/json"
	"testing"
)

func TestLearningToolArguments(t *testing.T) {
	// D4: input schemas forbid identity switching and arbitrary parameters.
	s := &Service{}
	for _, tool := range s.LearningTools() {
		if !tool.Annotations["readOnlyHint"].(bool) {
			t.Fatal(tool)
		}
		if e := mcp.ValidateArguments(tool.InputSchema, json.RawMessage(`{"subject":"math"}`)); e != nil {
			t.Fatal(e)
		}
		for _, raw := range []string{`{"subject":"all"}`, `{"subject":"math","class_id":2}`, `{"subject":"math","student_id":2}`, `{"subject":"math","role":"teacher"}`, `{}`} {
			if mcp.ValidateArguments(tool.InputSchema, json.RawMessage(raw)) == nil {
				t.Fatal("scope injection accepted", raw)
			}
		}
	}
}
