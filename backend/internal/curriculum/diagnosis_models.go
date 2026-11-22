package curriculum

import (
	"math"
	"sort"
	"time"
)

var learningZone = time.FixedZone("Asia/Shanghai", 8*60*60)

type DiagnosisFilter struct {
	Subject          string
	Days, Page, Size int
	StudentID        int64
	Start, End       time.Time
}

func (f *DiagnosisFilter) Validate(now time.Time) error {
	if !ValidSubject(f.Subject) || f.Days < 1 || f.Days > 180 || f.Page < 1 || f.Size < 1 || f.StudentID < 0 {
		return ErrInvalid
	}
	if f.Size > 100 {
		f.Size = 100
	}
	local := now.In(learningZone)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, learningZone)
	f.End = today.AddDate(0, 0, 1).UTC()
	f.Start = today.AddDate(0, 0, 1-f.Days).UTC()
	return nil
}

type LearningStats struct {
	Samples        int      `json:"samples"`
	Correct        int      `json:"correct"`
	Score          int      `json:"score"`
	MaxScore       int      `json:"max_score"`
	ScorePercent   *float64 `json:"score_percent"`
	CorrectPercent *float64 `json:"correct_percent"`
}

func (s *LearningStats) add(v DiagnosisSample) {
	s.Samples++
	if v.Correct {
		s.Correct++
	}
	s.Score += v.Score
	s.MaxScore += v.MaxScore
}
func percent(n, d int) *float64 {
	if d == 0 {
		return nil
	}
	v := math.Round(float64(n)*10000/float64(d)) / 100
	return &v
}
func (s *LearningStats) finish() {
	s.ScorePercent = percent(s.Score, s.MaxScore)
	s.CorrectPercent = percent(s.Correct, s.Samples)
}

type DiagnosisSample struct {
	SubmissionID  int64     `json:"submission_id"`
	AssignmentID  int64     `json:"assignment_id"`
	StudentID     int64     `json:"student_id"`
	StudentName   string    `json:"student_name"`
	Title         string    `json:"title"`
	Score         int       `json:"score"`
	MaxScore      int       `json:"max_score"`
	Correct       bool      `json:"correct"`
	ReviewVersion int       `json:"review_version"`
	SubmittedAt   time.Time `json:"submitted_at"`
	ReviewedAt    time.Time `json:"reviewed_at"`
	ErrorTags     []string  `json:"error_tags"`
	PointIDs      []int64   `json:"knowledge_point_ids"`
	Question      string    `json:"question,omitempty"`
	Answer        string    `json:"answer,omitempty"`
	Feedback      string    `json:"feedback,omitempty"`
}
type DiagnosisStudent struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
type PointStats struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	LearningStats
}
type DayStats struct {
	Date string `json:"date"`
	LearningStats
}
type DiagnosisReport struct {
	Subject     string `json:"subject"`
	From        string `json:"from"`
	To          string `json:"to"`
	Timezone    string `json:"timezone"`
	Fingerprint string `json:"fingerprint"`
	LearningStats
	StudentCount  int                `json:"student_count"`
	Students      []DiagnosisStudent `json:"students"`
	Points        []PointStats       `json:"points"`
	ErrorCounts   map[string]int     `json:"error_counts"`
	Trend         []DayStats         `json:"trend"`
	Evidence      []DiagnosisSample  `json:"evidence"`
	Page          int                `json:"page"`
	PageSize      int                `json:"page_size"`
	TotalEvidence int                `json:"total_evidence"`
}

func summarizeDiagnosis(f DiagnosisFilter, samples []DiagnosisSample, points []PointStats, students []DiagnosisStudent) DiagnosisReport {
	r := DiagnosisReport{Subject: f.Subject, From: f.Start.In(learningZone).Format("2006-01-02"), To: f.End.Add(-time.Nanosecond).In(learningZone).Format("2006-01-02"), Timezone: "Asia/Shanghai", Points: append([]PointStats{}, points...), Students: students, ErrorCounts: map[string]int{"concept": 0, "calculation": 0, "reading": 0, "method": 0, "careless": 0, "other": 0, "unclassified": 0}, Trend: []DayStats{}, Evidence: []DiagnosisSample{}, Page: f.Page, PageSize: f.Size, TotalEvidence: len(samples)}
	byPoint := map[int64]int{}
	for i, p := range r.Points {
		byPoint[p.ID] = i
	}
	byDay := map[string]*LearningStats{}
	seen := map[int64]bool{}
	for _, v := range samples {
		r.LearningStats.add(v)
		seen[v.StudentID] = true
		key := v.SubmittedAt.In(learningZone).Format("2006-01-02")
		if byDay[key] == nil {
			byDay[key] = &LearningStats{}
		}
		byDay[key].add(v)
		for _, id := range v.PointIDs {
			if i, ok := byPoint[id]; ok {
				r.Points[i].LearningStats.add(v)
			}
		}
		if !v.Correct {
			if len(v.ErrorTags) == 0 {
				r.ErrorCounts["unclassified"]++
			}
			for _, tag := range v.ErrorTags {
				r.ErrorCounts[tag]++
			}
		}
	}
	r.StudentCount = len(seen)
	r.finish()
	for i := range r.Points {
		r.Points[i].finish()
	}
	for day := f.Start; day.Before(f.End); day = day.AddDate(0, 0, 1) {
		key := day.In(learningZone).Format("2006-01-02")
		v := byDay[key]
		if v == nil {
			v = &LearningStats{}
		}
		v.finish()
		r.Trend = append(r.Trend, DayStats{key, *v})
	}
	// Fingerprints intentionally ignore pagination, but include membership and current source versions.
	r.Fingerprint = inputHash(struct {
		Subject, From, To string
		Student           int64
		Samples           []DiagnosisSample
		Points            []PointStats
		Students          []DiagnosisStudent
	}{f.Subject, r.From, r.To, f.StudentID, samples, r.Points, students})
	start := int64(f.Page-1) * int64(f.Size)
	if start < int64(len(samples)) {
		end := start + int64(f.Size)
		if end > int64(len(samples)) {
			end = int64(len(samples))
		}
		r.Evidence = append(r.Evidence, samples[start:end]...)
	}
	return r
}
func diagnosisSummary(r DiagnosisReport) map[string]any {
	points := append([]PointStats{}, r.Points...)
	sort.Slice(points, func(i, j int) bool {
		a, b := points[i].ScorePercent, points[j].ScorePercent
		if a == nil {
			return false
		}
		if b == nil {
			return true
		}
		if *a == *b {
			return points[i].ID < points[j].ID
		}
		return *a < *b
	})
	if len(points) > 20 {
		points = points[:20]
	}
	trend := r.Trend
	if len(trend) > 14 {
		trend = trend[len(trend)-14:]
	}
	return map[string]any{"totals": r.LearningStats, "points": points, "error_counts": r.ErrorCounts, "recent_trend": trend, "scope": "近期趋势与得分率较低的知识点，数值来自教师终评"}
}
