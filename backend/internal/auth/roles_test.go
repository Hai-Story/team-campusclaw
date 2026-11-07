package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStudentRoleEnforced(t *testing.T) {
	for _, role := range []string{"teacher", "student", ""} {
		called := false
		h := RequireStudent(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(200) })
		r := httptest.NewRequest("POST", "/api/conversations", nil)
		if role != "" {
			r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, User{Role: role}))
		}
		w := httptest.NewRecorder()
		h(w, r)
		want := 403
		if role == "student" {
			want = 200
		}
		if role == "" {
			want = 401
		}
		if w.Code != want || called != (role == "student") {
			t.Fatal(role, w.Code, called)
		}
	}
}
