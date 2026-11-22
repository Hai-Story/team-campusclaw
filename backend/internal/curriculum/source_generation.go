package curriculum

import (
	"campusclaw/backend/internal/auth"
	"context"
	"encoding/json"
	"sort"
	"time"
)

func (s *Service) generationEvidence(ctx context.Context, u auth.User, in SourceInput, query string) ([]Evidence, error) {
	if e := s.Store.CheckMaterials(ctx, u, in.MaterialIDs); e != nil {
		return nil, e
	}
	if s.Searcher == nil || s.Generator == nil {
		return nil, ErrModel
	}
	hits, e := s.Searcher.SearchBound(ctx, u.ClassID, query, in.MaterialIDs, 6)
	if e != nil {
		return nil, e
	}
	return s.Store.Evidence(ctx, u, hits)
}
func sourceData(refs []Evidence) []map[string]any {
	out := []map[string]any{}
	for _, r := range refs {
		out = append(out, map[string]any{"index": r.Index, "title": r.MaterialTitle, "text": r.Excerpt})
	}
	return out
}
func (s *Service) generationEvent(u auth.User, action, version, status string, start time.Time, hits int, usage *ModelUsage) {
	b, _ := json.Marshal(map[string]any{"time": time.Now().UTC().Format(time.RFC3339Nano), "level": "info", "module": "curriculum", "action": action, "actor_id": u.ID, "skill_id": version, "model": s.ModelName, "status": status, "duration_ms": time.Since(start).Milliseconds(), "hit_count": hits, "usage": usage})
	curriculumLog.Print(string(b))
}
func (s *Service) GenerateGraph(ctx context.Context, u auth.User, in SourceInput) (GraphGeneration, error) {
	if old, e := s.Store.GraphDraftByKey(ctx, u, in); e == nil {
		return GraphGeneration{Status: "generated", Draft: &old}, nil
	} else if e != ErrNotFound {
		return GraphGeneration{}, e
	}
	if !s.claim(u.ID) {
		return GraphGeneration{}, ErrBusy
	}
	defer s.release(u.ID)
	if old, e := s.Store.GraphDraftByKey(ctx, u, in); e == nil {
		return GraphGeneration{Status: "generated", Draft: &old}, nil
	} else if e != ErrNotFound {
		return GraphGeneration{}, e
	}
	start := time.Now()
	status := "failed"
	hits := 0
	var usage *ModelUsage
	defer func() {
		if ctx.Err() != nil {
			status = "cancelled"
		}
		s.generationEvent(u, "graph.generate", GraphVersion, status, start, hits, usage)
	}()
	budget, cancel := context.WithTimeout(ctx, 121*time.Second)
	defer cancel()
	refs, e := s.generationEvidence(budget, u, in, "提取讲义的知识点、章节以及先修、包含和关联关系")
	if e != nil {
		return GraphGeneration{}, e
	}
	hits = len(refs)
	if hits == 0 {
		status = "insufficient_evidence"
		return GraphGeneration{Status: status, Message: "指定资料中未找到可用依据，请补充讲义。"}, nil
	}
	catalog, e := s.Store.GraphCatalog(budget, u, in.Subject)
	if e != nil {
		return GraphGeneration{}, e
	}
	skill, e := teachingSkills.ReadFile("skills/knowledge-graph/SKILL.md")
	if e != nil {
		return GraphGeneration{}, e
	}
	names := []string{}
	for name := range catalog {
		names = append(names, name)
	}
	sort.Strings(names)
	text, used, e := s.Generator.Generate(budget, "graph-"+in.ClientKey, string(skill), map[string]any{"subject": in.Subject, "existing_nodes": names, "evidence": sourceData(refs)})
	usage = used
	if e != nil {
		return GraphGeneration{}, e
	}
	var out GraphOutput
	if e = strictJSON(text, &out); e != nil {
		return GraphGeneration{}, e
	}
	if out.Validate(catalog, len(refs)) != nil {
		return GraphGeneration{}, ErrModel
	}
	draft, e := s.Store.SaveGraphDraft(budget, u, in, out, refs, catalog)
	if e != nil {
		return GraphGeneration{}, e
	}
	status = "succeeded"
	return GraphGeneration{Status: "generated", Draft: &draft}, nil
}
