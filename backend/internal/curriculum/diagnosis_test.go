package curriculum

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDiagnosisCalendarAndCounting(t *testing.T) {
	// D2: China day boundary, weighted score, separate correctness and point overlap.
	now := time.Date(2026, 11, 14, 16, 30, 0, 0, time.UTC)
	f := DiagnosisFilter{Subject: "math", Days: 1, Page: 1, Size: 20}
	if f.Validate(now) != nil || f.Start.Format(time.RFC3339) != "2026-11-14T16:00:00Z" || f.End.Format(time.RFC3339) != "2026-11-15T16:00:00Z" {
		t.Fatal(f)
	}
	samples := []DiagnosisSample{{SubmissionID: 1, StudentID: 1, Score: 4, MaxScore: 10, Correct: false, SubmittedAt: now, ErrorTags: []string{"concept"}, PointIDs: []int64{1, 2}}, {SubmissionID: 2, StudentID: 2, Score: 20, MaxScore: 20, Correct: true, SubmittedAt: now, ErrorTags: []string{}, PointIDs: []int64{1}}}
	r := summarizeDiagnosis(f, samples, []PointStats{{ID: 1, Name: "共同点"}, {ID: 2, Name: "单点"}, {ID: 3, Name: "无样本"}}, []DiagnosisStudent{})
	if r.Samples != 2 || r.Score != 24 || r.MaxScore != 30 || *r.ScorePercent != 80 || *r.CorrectPercent != 50 || r.Points[0].Samples != 2 || r.Points[1].Samples != 1 || r.Points[2].ScorePercent != nil || r.ErrorCounts["concept"] != 1 || r.StudentCount != 2 || r.Trend[0].Samples != 2 {
		t.Fatal(r)
	}
	hash := r.Fingerprint
	f.Page = 2
	r = summarizeDiagnosis(f, samples, []PointStats{{ID: 1, Name: "共同点"}, {ID: 2, Name: "单点"}, {ID: 3, Name: "无样本"}}, []DiagnosisStudent{})
	if hash != r.Fingerprint || len(r.Evidence) != 0 {
		t.Fatal("pagination changed sample fingerprint")
	}
	samples[0].ReviewVersion++
	if hash == summarizeDiagnosis(f, samples, []PointStats{{ID: 1, Name: "共同点"}, {ID: 2, Name: "单点"}, {ID: 3, Name: "无样本"}}, []DiagnosisStudent{}).Fingerprint {
		t.Fatal("changed review was not reflected")
	}
	for _, v := range []DiagnosisFilter{{Subject: "all", Days: 1, Page: 1, Size: 20}, {Subject: "math", Days: 0, Page: 1, Size: 20}, {Subject: "math", Days: 181, Page: 1, Size: 20}} {
		if v.Validate(now) == nil {
			t.Fatal("invalid range accepted")
		}
	}
}
func TestDiagnosisExplanationNumbers(t *testing.T) {
	// D3: the model cannot introduce invented numeric claims or citations.
	r := DiagnosisReport{LearningStats: LearningStats{Samples: 2, Score: 4, MaxScore: 10}, Points: []PointStats{}, ErrorCounts: map[string]int{}}
	for _, text := range []string{"建议先回顾概念，再检查步骤。", "已确认2份样本。"} {
		if validateDiagnosisExplanation(text, r) != nil {
			t.Fatal(text)
		}
	}
	for _, text := range []string{"已有999999份样本。", "参考[999]。", "请访问https://example.com", " ", "得分为-4分。", "有4e10份样本。"} {
		if validateDiagnosisExplanation(text, r) == nil {
			t.Fatal("unsafe interpretation accepted", text)
		}
	}
}

func TestDiagnosisKnownLabelsAndDates(t *testing.T) {
	// D3: a known date/name is report data, while invented counts remain invalid.
	r := DiagnosisReport{LearningStats: LearningStats{Samples: 2}, Points: []PointStats{{Name: "Unit2026阅读"}}, Trend: []DayStats{{Date: "2026-11-14"}}}
	for _, text := range []string{"2026-11-14的终评样本较少。", "11月14日的样本较少。", "建议复习Unit2026阅读。"} {
		if validateDiagnosisExplanation(text, r) != nil {
			t.Fatal("known report label rejected", text)
		}
	}
	if validateDiagnosisExplanation("已有999999人参与。", r) == nil {
		t.Fatal("invented count accepted")
	}
}

func TestDiagnosisModelDataMinimization(t *testing.T) {
	// D3: only summary data reaches the model; student identity/answer fields stay out.
	r := DiagnosisReport{Students: []DiagnosisStudent{{ID: 99, Name: "PRIVATE_NAME"}}, Evidence: []DiagnosisSample{{StudentID: 99, Answer: "PRIVATE_ANSWER", Question: "PRIVATE_QUESTION"}}, Points: []PointStats{{ID: 1, Name: "公开知识点"}}}
	raw, e := json.Marshal(diagnosisSummary(r))
	if e != nil {
		t.Fatal(e)
	}
	for _, marker := range []string{"PRIVATE_NAME", "PRIVATE_ANSWER", "PRIVATE_QUESTION", "student_id", "student_name", "evidence"} {
		if strings.Contains(string(raw), marker) {
			t.Fatal("private field reached model", marker)
		}
	}
}
