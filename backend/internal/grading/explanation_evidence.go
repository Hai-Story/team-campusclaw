package grading

import (
	"campusclaw/backend/internal/retrieval"
	"campusclaw/backend/internal/subjects"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ExplanationSearcher interface {
	SearchBound(context.Context, int64, string, []int64, int) ([]retrieval.Hit, error)
}
type ExplanationCitation struct {
	Index int `json:"index"`
	retrieval.Hit
}
type ExplanationResponse struct {
	Explanation string                `json:"explanation"`
	Status      string                `json:"status"`
	Citations   []ExplanationCitation `json:"citations"`
}

func materialSubject(title string) string { return subjects.Classify(title) }
func (s *Store) ExplanationMaterials(ctx context.Context, w Work) ([]int64, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT id,title FROM materials WHERE class_id=? ORDER BY id`, w.ClassID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		var title string
		if e = rows.Scan(&id, &title); e != nil {
			return nil, e
		}
		if sub := materialSubject(title); sub == w.Subject || sub == "other" {
			out = append(out, id)
		}
	}
	return out, rows.Err()
}
func (s *Store) ExplanationEvidence(ctx context.Context, w Work, hits []retrieval.Hit) ([]ExplanationCitation, error) {
	out := []ExplanationCitation{}
	seen := map[int64]bool{}
	for _, h := range hits {
		if seen[h.ChunkID] {
			continue
		}
		seen[h.ChunkID] = true
		var v retrieval.Hit
		e := s.DB.QueryRowContext(ctx, `SELECT k.id,k.material_id,m.title,k.chunk_index,k.char_start,k.char_end,k.chunk_text FROM knowledge_chunks k JOIN materials m ON m.id=k.material_id AND m.class_id=k.class_id WHERE k.id=? AND k.material_id=? AND k.class_id=? AND k.index_status='ready'`, h.ChunkID, h.MaterialID, w.ClassID).Scan(&v.ChunkID, &v.MaterialID, &v.MaterialTitle, &v.ChunkIndex, &v.CharStart, &v.CharEnd, &v.Excerpt)
		if e == sql.ErrNoRows {
			return nil, ErrConflict
		}
		if e != nil {
			return nil, e
		}
		if sub := materialSubject(v.MaterialTitle); sub != w.Subject && sub != "other" {
			return nil, ErrConflict
		}
		out = append(out, ExplanationCitation{len(out) + 1, v})
	}
	return out, nil
}
func safeExplanationSources(refs []ExplanationCitation, expected string) []ExplanationCitation {
	out := append([]ExplanationCitation{}, refs...)
	for i := range out {
		if revealsAnswer(out[i].MaterialTitle, expected) {
			out[i].MaterialTitle = "班级资料"
		}
		if revealsAnswer(out[i].Excerpt, expected) {
			out[i].Excerpt = "相关片段正文已隐藏，请结合题目回顾概念。"
		}
	}
	return out
}
func ExplainEvidence(ctx context.Context, c Settings, work Work, refs []ExplanationCitation) (ExplanationResponse, Usage, *GradeError) {
	out := ExplanationResponse{Status: "generated", Citations: []ExplanationCitation{}}
	prompt, _ := promptFiles.ReadFile("prompts/explanation-v3.txt")
	safe := safeExplanationSources(refs, work.Expected)
	evidence := []map[string]any{}
	for _, r := range safe {
		evidence = append(evidence, map[string]any{"index": r.Index, "title": r.MaterialTitle, "text": r.Excerpt})
	}
	var text string
	var total Usage
	var failure *GradeError
	for attempt := 0; attempt < 2; attempt++ {
		single, cancel := context.WithTimeout(ctx, 60*time.Second)
		var used Usage
		text, used, failure = completion(single, c, "explain-"+strconv.FormatInt(work.SubmissionID, 10), string(prompt), map[string]any{"subject": work.Subject, "question": work.Question, "answer": work.Answer, "evidence": evidence}, 1024)
		cancel()
		total.PromptTokens += used.PromptTokens
		total.CompletionTokens += used.CompletionTokens
		if failure == nil || !failure.Retry || ctx.Err() != nil {
			break
		}
		if attempt == 0 {
			select {
			case <-ctx.Done():
				return out, total, failure
			case <-time.After(time.Second):
			}
		}
	}
	if failure != nil {
		return out, total, failure
	}
	var result struct {
		Explanation string `json:"explanation"`
		Indices     []int  `json:"citation_indices"`
	}
	d := json.NewDecoder(strings.NewReader(text))
	d.DisallowUnknownFields()
	e := d.Decode(&result)
	var extra any
	result.Explanation = strings.TrimSpace(result.Explanation)
	if e != nil || d.Decode(&extra) != io.EOF || len([]rune(result.Explanation)) < 1 || len([]rune(result.Explanation)) > 1500 || revealsAnswer(result.Explanation, work.Expected) || regexp.MustCompile(`\[\d+\]|https?://`).MatchString(result.Explanation) || len(result.Indices) < 1 || len(result.Indices) > len(safe) {
		return out, total, &GradeError{"explanation_invalid_response", false}
	}
	seen := map[int]bool{}
	for _, idx := range result.Indices {
		if idx < 1 || idx > len(safe) || seen[idx] {
			return out, total, &GradeError{"explanation_invalid_citation", false}
		}
		seen[idx] = true
		out.Citations = append(out.Citations, safe[idx-1])
	}
	out.Explanation = result.Explanation
	return out, total, nil
}
