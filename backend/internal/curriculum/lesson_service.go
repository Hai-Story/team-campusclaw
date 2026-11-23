package curriculum

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"log"
	"os"
	"time"

	"campusclaw/backend/internal/auth"
	"campusclaw/backend/internal/retrieval"
)

var curriculumLog = log.New(os.Stdout, "", 0)

var ErrEvidence = errors.New("资料已更新，请重新生成内容")
var ErrBusy = errors.New("正在生成，请稍后重试")

//go:embed skills/*/SKILL.md
var teachingSkills embed.FS

type EvidenceSearcher interface {
	SearchBound(context.Context, int64, string, []int64, int) ([]retrieval.Hit, error)
}
type LessonGeneration struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Draft   *LessonPlan `json:"draft"`
}

func (s *Service) claim(actor int64) bool {
	s.genMu.Lock()
	defer s.genMu.Unlock()
	if s.generating == nil {
		s.generating = map[int64]bool{}
	}
	if s.generating[actor] {
		return false
	}
	s.generating[actor] = true
	return true
}
func (s *Service) release(actor int64) { s.genMu.Lock(); delete(s.generating, actor); s.genMu.Unlock() }

func (s *Service) GenerateLesson(ctx context.Context, u auth.User, in LessonInput) (LessonGeneration, error) {
	result := LessonGeneration{}
	existing, err := s.Store.LessonByKey(ctx, u, in)
	if err == nil {
		return LessonGeneration{Status: "generated", Draft: &existing}, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return result, err
	}
	if !s.claim(u.ID) {
		return result, ErrBusy
	}
	defer s.release(u.ID)
	// Recheck after taking the actor slot, in case the previous generation just committed.
	if p, e := s.Store.LessonByKey(ctx, u, in); e == nil {
		return LessonGeneration{Status: "generated", Draft: &p}, nil
	} else if !errors.Is(e, ErrNotFound) {
		return result, e
	}
	start := time.Now()
	status := "failed"
	var usage *ModelUsage
	hitCount := 0
	defer func() {
		if ctx.Err() != nil {
			status = "cancelled"
		}
		event := map[string]any{"time": time.Now().UTC().Format(time.RFC3339Nano), "level": "info", "hit_count": hitCount, "module": "curriculum", "action": "lesson.generate", "actor_id": u.ID, "skill_id": LessonVersion, "model": s.ModelName, "status": status, "duration_ms": time.Since(start).Milliseconds(), "usage": usage}
		raw, _ := json.Marshal(event)
		curriculumLog.Print(string(raw))
	}()
	if err = s.Store.CheckMaterials(ctx, u, in.MaterialIDs); err != nil {
		return result, err
	}
	if s.Searcher == nil || s.Generator == nil {
		return result, ErrModel
	}
	budget, cancel := context.WithTimeout(ctx, 121*time.Second)
	defer cancel()
	hits, err := s.Searcher.SearchBound(budget, u.ClassID, in.Chapter+" "+in.Objectives, in.MaterialIDs, 6)
	if err != nil {
		return result, err
	}
	hitCount = len(hits)
	if len(hits) == 0 {
		status = "insufficient_evidence"
		return LessonGeneration{Status: status, Message: "本班指定资料中未找到足够依据，请调整章节或补充讲义。", Draft: nil}, nil
	}
	refs, err := s.Store.Evidence(budget, u, hits)
	if err != nil {
		return result, err
	}
	evidenceInput := []map[string]any{}
	for _, e := range refs {
		evidenceInput = append(evidenceInput, map[string]any{"index": e.Index, "title": e.MaterialTitle, "text": e.Excerpt})
	}
	skill, err := teachingSkills.ReadFile("skills/lesson-plan/SKILL.md")
	if err != nil {
		return result, err
	}
	text, used, err := s.Generator.Generate(budget, "lesson-"+in.Hash()[:16]+"-"+in.ClientKey, string(skill), map[string]any{"subject": in.Subject, "chapter": in.Chapter, "objectives": in.Objectives, "total_minutes": in.Periods * in.MinutesPerPeriod, "evidence": evidenceInput})
	usage = used
	if err != nil {
		return result, err
	}
	var generated LessonOutput
	if err = strictJSON(text, &generated); err != nil {
		return result, err
	}
	if err = generated.Validate(in.Periods*in.MinutesPerPeriod, len(refs)); err != nil {
		return result, err
	}
	selected := []Evidence{}
	for _, i := range generated.CitationIndices {
		selected = append(selected, refs[i-1])
	}
	body := generated.Body()
	if len([]rune(body)) > 20000 {
		return result, ErrModel
	}
	if budget.Err() != nil {
		return result, budget.Err()
	}
	p, err := s.Store.CreateLesson(budget, u, in, body, selected)
	if err != nil {
		return result, err
	}
	status = "succeeded"
	return LessonGeneration{Status: "generated", Draft: &p}, nil
}
