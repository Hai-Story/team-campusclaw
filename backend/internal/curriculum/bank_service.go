package curriculum

import (
	"campusclaw/backend/internal/auth"
	"context"
	"strings"
	"time"
)

func (s *Service) GenerateQuestions(ctx context.Context, u auth.User, in BankInput) (BankGeneration, error) {
	if old, e := s.Store.QuestionDraftByKey(ctx, u, in); e == nil {
		return BankGeneration{Status: "generated", Draft: &old}, nil
	} else if e != ErrNotFound {
		return BankGeneration{}, e
	}
	if !s.claim(u.ID) {
		return BankGeneration{}, ErrBusy
	}
	defer s.release(u.ID)
	if old, e := s.Store.QuestionDraftByKey(ctx, u, in); e == nil {
		return BankGeneration{Status: "generated", Draft: &old}, nil
	} else if e != ErrNotFound {
		return BankGeneration{}, e
	}
	start := time.Now()
	status := "failed"
	hits := 0
	var usage *ModelUsage
	defer func() {
		if ctx.Err() != nil {
			status = "cancelled"
		}
		s.generationEvent(u, "bank.generate", BankVersion, status, start, hits, usage)
	}()
	budget, cancel := context.WithTimeout(ctx, 121*time.Second)
	defer cancel()
	points, e := s.Store.BankPoints(budget, u, in.Subject, in.KnowledgePointIDs)
	if e != nil {
		return BankGeneration{}, e
	}
	names := []string{}
	meta := []map[string]any{}
	for _, p := range points {
		names = append(names, p.Name)
		meta = append(meta, map[string]any{"id": p.ID, "name": p.Name, "chapter": p.Chapter, "description": p.Description})
	}
	refs, e := s.generationEvidence(budget, u, in.SourceInput, strings.Join(names, " ")+" 设计练习题")
	if e != nil {
		return BankGeneration{}, e
	}
	hits = len(refs)
	if hits == 0 {
		status = "insufficient_evidence"
		return BankGeneration{Status: status, Message: "指定资料中未找到可用依据，请补充讲义。"}, nil
	}
	avoid, e := s.Store.BankStems(budget, u, in.Subject)
	if e != nil {
		return BankGeneration{}, e
	}
	skill, e := teachingSkills.ReadFile("skills/question-generation/SKILL.md")
	if e != nil {
		return BankGeneration{}, e
	}
	text, used, e := s.Generator.Generate(budget, "bank-"+in.ClientKey, string(skill), map[string]any{"subject": in.Subject, "knowledge_points": meta, "question_types": in.QuestionTypes, "difficulty": in.Difficulty, "question_count": in.QuestionCount, "evidence": sourceData(refs), "avoid_stems": avoid})
	usage = used
	if e != nil {
		return BankGeneration{}, e
	}
	var out struct {
		Questions []QuestionItem `json:"questions"`
	}
	if e = strictJSON(text, &out); e != nil {
		return BankGeneration{}, e
	}
	if validateQuestions(out.Questions, in, len(refs), true) != nil {
		return BankGeneration{}, ErrModel
	}
	draft, e := s.Store.SaveQuestionDraft(budget, u, in, out.Questions, refs)
	if e != nil {
		return BankGeneration{}, e
	}
	status = "succeeded"
	return BankGeneration{Status: "generated", Draft: &draft}, nil
}
