package auditlog

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestMetadataBoundary(t *testing.T) {
	for _, s := range []string{"secret\nforge", "<script>", strings.Repeat("x", 201), "Bearer token=xxx"} {
		if Safe(s) != "unknown" {
			t.Fatal("unsafe value accepted")
		}
	}
	if Safe("POST /api/submissions/{id}/review") == "unknown" {
		t.Fatal("route rejected")
	}
	if Bind(context.Background(), 1, 1) != nil {
		t.Fatal("optional trace failed")
	}
	v, _ := json.Marshal(Event{Action: "GET /api/me"})
	if strings.Contains(string(v), "password") {
		t.Fatal("unexpected data")
	}
}
