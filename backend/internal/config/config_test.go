package config

import (
	"os"
	"strings"
	"testing"
)

func validEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{"APP_VERSION", "API_PORT", "WEB_PORT", "MYSQL_HOST", "MYSQL_PORT", "MYSQL_DATABASE", "MYSQL_USER", "MYSQL_PASSWORD", "SESSION_SECRET", "SEED_TEACHER_PASSWORD", "SEED_STUDENT_PASSWORD", "UPLOAD_DIR", "MAX_UPLOAD_BYTES", "SESSION_TTL_HOURS", "LOGIN_FAIL_LIMIT", "LOGIN_FAIL_WINDOW_SECONDS", "EMBEDDING_BASE_URL", "EMBEDDING_API_KEY", "EMBEDDING_MODEL", "EMBEDDING_DIM", "CHAT_BASE_URL", "CHAT_API_KEY", "CHAT_MODEL", "QDRANT_URL", "CHUNK_AUTO_MAX_CHARS", "CHUNK_AUTO_OVERLAP", "VECTOR_SCORE_THRESHOLD", "RRF_K", "ASK_TOP_K", "MCP_AUTH_TOKEN", "MCP_ENCRYPTION_KEY", "MCP_ALLOWED_BASE_URLS", "MCP_MAX_ROUNDS", "MCP_MAX_CALLS_PER_TURN", "MCP_MAX_CALLS_PER_CONVERSATION"} {
		t.Setenv(key, "1")
	}
	t.Setenv("SESSION_SECRET", strings.Repeat("x", 32))
	t.Setenv("CHUNK_AUTO_MAX_CHARS", "800")
	t.Setenv("CHUNK_AUTO_OVERLAP", "80")
	t.Setenv("VECTOR_SCORE_THRESHOLD", "0.25")
}

// team-runtime-foundation S2/S3: missing and illegal configuration cannot fall back.
func TestRejectConfiguration(t *testing.T) {
	for _, tc := range []struct{ key, value string }{{"SESSION_SECRET", ""}, {"SESSION_SECRET", "short"}, {"API_PORT", "70000"}, {"WEB_PORT", "-1"}, {"MYSQL_PORT", "abc"}, {"VECTOR_SCORE_THRESHOLD", "NaN"}, {"CHUNK_AUTO_OVERLAP", "800"}} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			validEnvironment(t)
			t.Setenv(tc.key, tc.value)
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("%s not rejected", tc.key)
			}
		})
	}
}
func TestFullConfiguration(t *testing.T) {
	validEnvironment(t)
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	_ = os.Getenv("APP_VERSION")
}
