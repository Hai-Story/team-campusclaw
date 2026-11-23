package curriculum

import (
	"campusclaw/backend/internal/auth"
	"context"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const DiagnosisVersion = "learning-diagnosis-v2"

var diagnosisNumber = regexp.MustCompile(`[-+]?\d+(?:\.\d+)?(?:[eE][-+]?\d+)?`)

func numberKey(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
func diagnosisNumbers(r DiagnosisReport) map[string]bool {
	out := map[string]bool{}
	add := func(s LearningStats) {
		for _, v := range []int{s.Samples, s.Correct, s.Score, s.MaxScore} {
			out[numberKey(float64(v))] = true
		}
		for _, v := range []*float64{s.ScorePercent, s.CorrectPercent} {
			if v != nil {
				out[numberKey(*v)] = true
			}
		}
	}
	// Calendar components are already report data; keep them distinct from negative scores.
	for _, day := range r.Trend {
		if d, e := time.Parse("2006-01-02", day.Date); e == nil {
			for _, v := range []int{d.Year(), int(d.Month()), d.Day()} {
				out[numberKey(float64(v))] = true
			}
		}
	}
	add(r.LearningStats)
	for _, p := range r.Points {
		add(p.LearningStats)
	}
	for _, v := range r.Trend {
		add(v.LearningStats)
	}
	for _, v := range r.ErrorCounts {
		out[numberKey(float64(v))] = true
	}
	return out
}
func validateDiagnosisExplanation(text string, r DiagnosisReport) error {
	if len([]rune(strings.TrimSpace(text))) < 1 || len([]rune(text)) > 2000 || regexp.MustCompile(`\[\d+\]|https?://`).MatchString(text) {
		return ErrModel
	}
	numbers := diagnosisNumbers(r)
	checked := text
	for _, day := range r.Trend {
		if d, e := time.Parse("2006-01-02", day.Date); e == nil {
			for _, format := range []string{"2006-01-02", "2006年1月2日", "1月2日", "2006/1/2"} {
				checked = strings.ReplaceAll(checked, d.Format(format), "")
			}
		}
	}
	for _, p := range r.Points {
		if len([]rune(p.Name)) > 1 {
			if _, e := strconv.ParseFloat(p.Name, 64); e != nil {
				checked = strings.ReplaceAll(checked, p.Name, "")
			}
		}
	}
	for _, token := range diagnosisNumber.FindAllString(checked, -1) {
		n, e := strconv.ParseFloat(token, 64)
		if e != nil || n < 0 || !numbers[numberKey(n)] {
			return ErrModel
		}
	}
	return nil
}

type DiagnosisExplanation struct {
	Status      string `json:"status"`
	Explanation string `json:"explanation"`
	Fingerprint string `json:"fingerprint"`
}

func (s *Service) ExplainDiagnosis(ctx context.Context, u auth.User, f DiagnosisFilter, fingerprint string) (DiagnosisExplanation, error) {
	budget, cancel := context.WithTimeout(ctx, 121*time.Second)
	defer cancel()
	empty := DiagnosisExplanation{}
	if !s.claim(u.ID) {
		return empty, ErrBusy
	}
	defer s.release(u.ID)
	report, e := s.Store.Diagnosis(budget, u, f)
	if e != nil {
		return empty, e
	}
	if report.Fingerprint != fingerprint {
		return empty, ErrConflict
	}
	if report.Samples == 0 {
		return DiagnosisExplanation{"insufficient_data", "当前范围暂无教师终评样本，终评完成后可查看分析。", fingerprint}, nil
	}
	if s.Generator == nil {
		return empty, ErrModel
	}
	start := time.Now()
	status := "failed"
	var usage *ModelUsage
	defer func() {
		if ctx.Err() != nil {
			status = "cancelled"
		}
		s.generationEvent(u, "diagnosis.explain", DiagnosisVersion, status, start, 0, usage)
	}()
	prompt, e := teachingSkills.ReadFile("skills/learning-diagnosis/SKILL.md")
	if e != nil {
		return empty, e
	}
	data := diagnosisSummary(report)
	numbers := diagnosisNumbers(report)
	allowed := []string{}
	for n := range numbers {
		allowed = append(allowed, n)
	}
	sort.Strings(allowed)
	data["allowed_numbers"] = allowed
	text, used, e := s.Generator.Generate(budget, "diagnosis-"+fingerprint[:16], string(prompt), data)
	usage = used
	if e != nil {
		return empty, e
	}
	var result struct {
		Explanation string `json:"explanation"`
	}
	if e = strictJSON(text, &result); e != nil {
		return empty, e
	}
	result.Explanation = strings.TrimSpace(result.Explanation)
	if e = validateDiagnosisExplanation(result.Explanation, report); e != nil {
		return empty, e
	}
	current, e := s.Store.Diagnosis(budget, u, f)
	if e != nil {
		return empty, e
	}
	if current.Fingerprint != fingerprint {
		status = "stale"
		return empty, ErrConflict
	}
	status = "succeeded"
	return DiagnosisExplanation{"generated", result.Explanation, fingerprint}, nil
}
