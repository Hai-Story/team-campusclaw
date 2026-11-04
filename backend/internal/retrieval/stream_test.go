package retrieval

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRealStreamAndTruncatedFailure(t *testing.T) {
	for _, done := range []bool{true, false} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var in map[string]any
			json.NewDecoder(r.Body).Decode(&in)
			if in["stream"] != true {
				t.Error("not streaming")
			}
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"先\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"思考\"}}]}\n\n")
			if done {
				fmt.Fprint(w, "data: [DONE]\n\n")
			}
		}))
		c := ChatClient{BaseURL: srv.URL}
		got := []string{}
		e := c.Stream(context.Background(), nil, func(s string) error { got = append(got, s); return nil })
		srv.Close()
		if len(got) != 2 || got[1] != "思考" || (e == nil) != done {
			t.Fatal(got, e)
		}
	}
}
func TestQdrantMaterialFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			fmt.Fprint(w, `{"result":{}}`)
			return
		}
		var in map[string]any
		json.NewDecoder(r.Body).Decode(&in)
		must := in["filter"].(map[string]any)["must"].([]any)
		if len(must) != 2 {
			t.Error("missing material filter")
		}
		fmt.Fprint(w, `{"result":[]}`)
	}))
	defer srv.Close()
	q := &QdrantClient{BaseURL: srv.URL}
	_, e := q.SearchMaterials(context.Background(), 1, []float32{1}, 4, 0.25, []int64{7})
	if e != nil {
		t.Fatal(e)
	}
}
