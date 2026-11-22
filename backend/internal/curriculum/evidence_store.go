package curriculum

import (
	"campusclaw/backend/internal/auth"
	"context"
	"encoding/json"
)

// resolveReferences deduplicates source checks within a response without caching across users or requests.
func resolveReferences(ctx context.Context, q queryer, u auth.User, refs []Evidence, cache map[string][]Evidence) ([]Evidence, error) {
	raw, _ := json.Marshal(refs)
	key := string(raw)
	if value, ok := cache[key]; ok {
		return value, nil
	}
	value, e := currentEvidenceWith(ctx, q, u, refs)
	if e != nil {
		return nil, e
	}
	cache[key] = value
	return value, nil
}
