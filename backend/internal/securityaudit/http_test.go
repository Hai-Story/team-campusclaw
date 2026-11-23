package securityaudit

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestQueryBounds(t *testing.T) {
	now := time.Now().UTC()
	for _, q := range []string{"page=0", "page=1.5", "page_size=-1", "actor_id=x", "skill_id=%0Afoo", "sort=body", "result=secret", "from=2026-01-01T00:00:00Z", "request_id=key", "page=1&page=2", "to=invalid", "before_id=0"} {
		if _, ok := parse(httptest.NewRequest("GET", "/api/audit-events?"+q, nil), now); ok {
			t.Fatalf("accepted %s", q)
		}
	}
	f, ok := parse(httptest.NewRequest("GET", "/api/audit-events?page_size=101&skill_id=grading-verdict-v3", nil), now)
	if !ok || f.Size != 100 || f.Page != 1 {
		t.Fatal(f, ok)
	}
}
