package curriculum

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// K1-S2/K2-S1: validate actual accepted/rejected inputs, including zero/false boundaries.
func TestPointValidation(t *testing.T) {
	for _, subject := range []string{"chinese", "math", "english", "science", "morality"} {
		in := PointInput{Subject: subject, Name: " 分数 ", MaterialIDs: []int64{1}}
		if err := in.Validate(false); err != nil || in.Name != "分数" {
			t.Fatal(in, err)
		}
	}
	invalid := []PointInput{{Subject: "all", Name: "分数"}, {Subject: "math", Name: " "}, {Subject: "math", Name: strings.Repeat("字", 121)}, {Subject: "math", Name: "分数", AssignmentIDs: []int64{1, 1}}, {Subject: "math", Name: "分数", MaterialIDs: []int64{0}}, {Subject: "math", Name: "分数", MaterialIDs: make([]int64, 51)}}
	for _, in := range invalid {
		if in.Validate(false) == nil {
			t.Fatal("accepted invalid input", in)
		}
	}
	in := PointInput{Subject: "math", Name: "分数"}
	if in.Validate(true) == nil {
		t.Fatal("missing version")
	}
	v := 1
	in.ExpectedVersion = &v
	if in.Validate(true) != nil || in.Validate(false) == nil {
		t.Fatal("version semantics")
	}
}
func TestPointFilter(t *testing.T) {
	for _, q := range []string{"?subject=all", "?page=0", "?page=1.5", "?page=-1", "?page_size=0", "?favorites=1", "?subject=math&subject=english", "?role=teacher", "?q=" + strings.Repeat("a", 101)} {
		if _, e := pointFilter(httptest.NewRequest("GET", "/api/knowledge-points"+q, nil)); e == nil {
			t.Fatal(q)
		}
	}
	f, e := pointFilter(httptest.NewRequest("GET", "/api/knowledge-points?page_size=101&favorites=false", nil))
	if e != nil || f.Size != 100 || f.Page != 1 || f.Subject != "math" || f.Favorites {
		t.Fatal(f, e)
	}
}
func TestStrictBodyAndUnauthenticated(t *testing.T) {
	for _, body := range []string{`{"subject":"math","name":"x","role":"teacher"}`, `null`, `{} {}`} {
		r := httptest.NewRequest("POST", "/api/knowledge-points", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		var in PointInput
		if decode(w, r, &in) && in.Validate(false) == nil {
			t.Fatal("accepted invalid body", body)
		}
	}
	mux := &Service{}
	w := httptest.NewRecorder()
	mux.ListPoints(w, httptest.NewRequest("GET", "/api/knowledge-points", nil))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	for err, status := range map[error]int{ErrInvalid: 400, ErrConflict: 409, ErrNotFound: 404} {
		w = httptest.NewRecorder()
		failure(w, err)
		if w.Code != status {
			t.Fatal(w.Code, status)
		}
	}
}
