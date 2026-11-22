package mcp

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNativeCallBudget(t *testing.T) {
	// D4: the fixed one-minute budget is per authenticated actor.
	m := &Manager{}
	now := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 30; i++ {
		if !m.nativePermit(1, now) {
			t.Fatal("early limit", i)
		}
	}
	if m.nativePermit(1, now.Add(59*time.Second)) {
		t.Fatal("budget bypass")
	}
	if !m.nativePermit(2, now) || !m.nativePermit(1, now.Add(time.Minute)) {
		t.Fatal("independent/reset budget failed")
	}
}
func TestLearningFingerprintChange(t *testing.T) {
	// D4: approval must match the running definition rather than a stale cache.
	tool := Tool{Name: "knowledge_points", Description: "old", InputSchema: json.RawMessage(`{"type":"object"}`)}
	old := fingerprint(tool)
	tool.Description = "new"
	if old == fingerprint(tool) {
		t.Fatal("definition change not tracked")
	}
}
