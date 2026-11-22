package curriculum

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const GraphVersion = "knowledge-graph-v1"

type SourceInput struct {
	Subject     string  `json:"subject"`
	MaterialIDs []int64 `json:"material_ids"`
	ClientKey   string  `json:"client_key"`
}

func (in SourceInput) Validate() error {
	if !ValidSubject(in.Subject) || len(in.MaterialIDs) < 1 || len(in.MaterialIDs) > 10 || !validIDs(in.MaterialIDs) || !clientKey.MatchString(in.ClientKey) {
		return ErrInvalid
	}
	return nil
}
func (in SourceInput) Hash() string { in.ClientKey = ""; return inputHash(in) }
func inputHash(in any) string {
	b, _ := json.Marshal(in)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func relationKind(kind string) bool {
	return kind == "prerequisite" || kind == "contains" || kind == "related"
}

type GraphEdge struct {
	ID      int64  `json:"id"`
	FromID  int64  `json:"from_id"`
	ToID    int64  `json:"to_id"`
	Kind    string `json:"kind"`
	Version int    `json:"version"`
}
type EdgeInput struct {
	FromID int64  `json:"from_id"`
	ToID   int64  `json:"to_id"`
	Kind   string `json:"kind"`
}

func (e *EdgeInput) Validate() error {
	if e.FromID < 1 || e.ToID < 1 || e.FromID == e.ToID || !relationKind(e.Kind) {
		return ErrInvalid
	}
	if e.Kind == "related" && e.FromID > e.ToID {
		e.FromID, e.ToID = e.ToID, e.FromID
	}
	return nil
}
func createsCycle(edges []GraphEdge, from, to int64, kind string) bool {
	if from == to {
		return true
	}
	if kind == "related" {
		return false
	}
	next := map[int64][]int64{}
	for _, e := range edges {
		if e.Kind == kind {
			next[e.FromID] = append(next[e.FromID], e.ToID)
		}
	}
	seen := map[int64]bool{}
	stack := []int64{to}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if id == from {
			return true
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		stack = append(stack, next[id]...)
	}
	return false
}

type Mastery struct {
	Reviewed  int  `json:"reviewed"`
	Incorrect int  `json:"incorrect"`
	Weak      bool `json:"weak"`
}
type GraphNode struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Chapter string `json:"chapter"`
	Mastery
}
type Graph struct {
	Subject string      `json:"subject"`
	Nodes   []GraphNode `json:"nodes"`
	Edges   []GraphEdge `json:"edges"`
}
type CandidateNode struct {
	Name        string `json:"name"`
	Chapter     string `json:"chapter"`
	Description string `json:"description"`
}
type CandidateEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}
type GraphOutput struct {
	Nodes           []CandidateNode `json:"nodes"`
	Edges           []CandidateEdge `json:"edges"`
	CitationIndices []int           `json:"citation_indices"`
}

func (o *GraphOutput) Validate(catalog map[string]int64, refs int) error {
	if o.Edges == nil || len(o.Nodes) < 1 || len(o.Nodes) > 10 || len(o.Edges) > 20 || len(o.CitationIndices) < 1 || len(o.CitationIndices) > refs {
		return ErrInvalid
	}
	known := map[string]bool{}
	for name := range catalog {
		known[strings.ToLower(name)] = true
	}
	added := map[string]bool{}
	for i := range o.Nodes {
		n := &o.Nodes[i]
		n.Name = strings.TrimSpace(n.Name)
		n.Chapter = strings.TrimSpace(n.Chapter)
		n.Description = strings.TrimSpace(n.Description)
		key := strings.ToLower(n.Name)
		if len([]rune(n.Name)) < 1 || len([]rune(n.Name)) > 120 || len([]rune(n.Chapter)) > 120 || len([]rune(n.Description)) > 2000 || added[key] {
			return ErrInvalid
		}
		known[key] = true
		added[key] = true
	}
	used := map[string]bool{}
	for i := range o.Edges {
		e := &o.Edges[i]
		e.From = strings.TrimSpace(e.From)
		e.To = strings.TrimSpace(e.To)
		from, to := strings.ToLower(e.From), strings.ToLower(e.To)
		if !known[from] || !known[to] || from == to || !relationKind(e.Kind) {
			return ErrInvalid
		}
		if e.Kind == "related" && from > to {
			e.From, e.To = e.To, e.From
			from, to = to, from
		}
		key := from + "\x00" + to + "\x00" + e.Kind
		if used[key] {
			return ErrInvalid
		}
		used[key] = true
	}
	indices := map[int]bool{}
	for _, i := range o.CitationIndices {
		if i < 1 || i > refs || indices[i] {
			return ErrInvalid
		}
		indices[i] = true
	}

	for _, n := range o.Nodes {
		for _, match := range generatedCitation.FindAllStringSubmatch(n.Name+n.Chapter+n.Description, -1) {
			var index int
			for _, i := range o.CitationIndices {
				if match[1] == fmt.Sprint(i) {
					index = i
				}
			}
			if index == 0 {
				return ErrInvalid
			}
		}
	}
	return nil
}

type GraphDraft struct {
	Existing map[string]int64 `json:"existing_nodes"`
	ID       int64            `json:"id"`
	Subject  string           `json:"subject"`
	Input    SourceInput      `json:"input"`
	GraphOutput
	Citations []Evidence `json:"citations"`
	PointIDs  []int64    `json:"point_ids"`
	Status    string     `json:"status"`
	Version   int        `json:"version"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
type GraphUpdate struct {
	GraphOutput
	ExpectedVersion int `json:"expected_version"`
}
type GraphGeneration struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Draft   *GraphDraft `json:"draft"`
}
