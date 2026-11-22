package curriculum

import "testing"

// G1/G2: cycles, canonical undirected edges and candidate endpoint validation.
func TestGraphRules(t *testing.T) {
	edges := []GraphEdge{{FromID: 1, ToID: 2, Kind: "prerequisite"}, {FromID: 2, ToID: 3, Kind: "prerequisite"}}
	if !createsCycle(edges, 3, 1, "prerequisite") || createsCycle(edges, 1, 3, "prerequisite") || createsCycle(edges, 3, 1, "related") {
		t.Fatal("cycle boundary")
	}
	in := EdgeInput{FromID: 3, ToID: 1, Kind: "related"}
	if in.Validate() != nil || in.FromID != 1 {
		t.Fatal("canonical")
	}
	in.ToID = 1
	if in.Validate() == nil {
		t.Fatal("self edge")
	}
	o := GraphOutput{Nodes: []CandidateNode{{Name: "新节点"}}, Edges: []CandidateEdge{{From: "已有", To: "新节点", Kind: "prerequisite"}}, CitationIndices: []int{1}}
	if o.Validate(map[string]int64{"已有": 1}, 1) != nil {
		t.Fatal("valid")
	}
	o.Edges[0].From = "不存在"
	if o.Validate(nil, 1) == nil {
		t.Fatal("invented endpoint")
	}
}
