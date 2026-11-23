package runtimehttp

import (
	"campusclaw/backend/internal/auditlog"
	"campusclaw/backend/internal/observability"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type requestKey struct{}

func ID(ctx context.Context) string { value, _ := ctx.Value(requestKey{}).(string); return value }

type response struct {
	http.ResponseWriter
	status int
}

func (r *response) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
		r.ResponseWriter.WriteHeader(status)
	}
}
func (r *response) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.WriteHeader(200)
	}
	return r.ResponseWriter.Write(data)
}
func (r *response) Flush() {
	if r.status == 0 {
		r.WriteHeader(200)
	}
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
func Middleware(next http.Handler, stores ...*auditlog.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 16)
		if _, err := rand.Read(buf); err != nil {
			http.Error(w, "服务暂时不可用", 503)
			return
		}
		id := hex.EncodeToString(buf)
		w.Header().Set("X-Request-ID", id)
		start := time.Now()
		rec := &response{ResponseWriter: w}
		audit := observability.New()
		ctx := observability.With(context.WithValue(r.Context(), requestKey{}, id), audit)
		var trace *auditlog.Trace
		if len(stores) > 0 && strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/api/openapi.json" {
			call, cancel := context.WithTimeout(ctx, 3*time.Second)
			var err error
			route := r.Method + " unknown"
			if mux, ok := next.(*http.ServeMux); ok {
				_, pattern := mux.Handler(r)
				if pattern != "" {
					route = pattern
				}
			}
			trace, err = stores[0].Start(call, "http:"+id, auditlog.Event{RequestID: id, Action: route, Object: "collection"})
			cancel()
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(503)
				json.NewEncoder(w).Encode(map[string]string{"error": "审计服务暂不可用，请稍后重试"})
				return
			}
			ctx = auditlog.With(ctx, trace)
		}
		req := r.WithContext(ctx)
		finished := false
		defer func() {
			if trace == nil {
				return
			}
			status := rec.status
			if status == 0 {
				status = 200
			}
			if !finished {
				status = 500
			}
			outcome := "succeeded"
			if status >= 400 {
				outcome = "failed"
			}
			if status == 401 || status == 403 || status == 404 {
				outcome = "denied"
			}
			if r.Context().Err() != nil {
				outcome = "cancelled"
			}
			fields := audit.Fields()
			var skill any
			if fields != nil {
				skill = fields["skill_id"]
				if status < 400 && (fields["status"] == "failed" || fields["status"] == "cancelled") {
					outcome = fields["status"].(string)
				}
			}
			route := req.Pattern
			if route == "" {
				route = r.Method + " unknown"
			}
			call, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
			defer cancel()
			if err := trace.Finish(call, route, auditlog.Object(req.PathValue), outcome, status, time.Since(start).Milliseconds(), fields, skill); err != nil {
				fmt.Fprintln(os.Stdout, `{"module":"audit","event":"completion_failed","request_id":"`+id+`"}`)
			}
		}()
		next.ServeHTTP(rec, req)
		finished = true
		if fields := audit.Fields(); fields != nil {
			fields["ts"] = time.Now().UTC().Format(time.RFC3339Nano)
			fields["level"] = "info"
			fields["module"] = "tutor"
			fields["event"] = "generation"
			fields["request_id"] = id
			fields["duration_ms"] = time.Since(start).Milliseconds()
			if r.Context().Err() != nil {
				fields["status"] = "cancelled"
			}
			raw, _ := json.Marshal(fields)
			fmt.Fprintln(os.Stdout, string(raw))
		}
		status := rec.status
		if status == 0 {
			status = 200
		}
		entry, _ := json.Marshal(map[string]any{"ts": time.Now().UTC().Format(time.RFC3339Nano), "level": "info", "module": "http", "request_id": id, "method": r.Method, "path": r.URL.EscapedPath(), "status": status, "duration_ms": time.Since(start).Milliseconds()})
		fmt.Fprintln(os.Stdout, string(entry))
	})
}

// @api {"method":"GET","path":"/health","summary":"聚合就绪检查","auth":"public","status":200,"response":{"type":"object"},"errors":{"400":"查询参数非法","405":"方法不允许","503":"依赖暂不可用"}}
// @api {"method":"GET","path":"/live","summary":"进程存活检查","auth":"public","status":200,"response":{"type":"object"},"errors":{"400":"查询参数非法","405":"方法不允许"}}
func Probes(mux *http.ServeMux, db *sql.DB, version, searchURL, toolsURL string) {
	for _, path := range []string{"/health", "/live"} {
		path := path
		mux.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if r.URL.RawQuery != "" {
				w.WriteHeader(400)
				json.NewEncoder(w).Encode(map[string]string{"error": "探针不接受查询参数", "request_id": ID(r.Context())})
				return
			}
			out := map[string]any{"status": "alive", "service": "campusclaw", "version": version, "time": time.Now().UTC().Format(time.RFC3339Nano), "request_id": ID(r.Context())}
			status := 200
			if path == "/health" {
				deps := map[string]string{"storage": "down", "search": "down", "tools": "down"}
				var mu sync.Mutex
				var wg sync.WaitGroup
				checks := map[string]func(context.Context) bool{"storage": func(ctx context.Context) bool { return db != nil && db.PingContext(ctx) == nil }, "search": func(ctx context.Context) bool { return ping(ctx, strings.TrimRight(searchURL, "/")+"/healthz") }, "tools": func(ctx context.Context) bool { return ping(ctx, strings.TrimRight(toolsURL, "/")+"/health") }}
				for name, fn := range checks {
					wg.Add(1)
					go func(name string, fn func(context.Context) bool) {
						defer wg.Done()
						ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
						defer cancel()
						if fn(ctx) {
							mu.Lock()
							deps[name] = "up"
							mu.Unlock()
						}
					}(name, fn)
				}
				wg.Wait()
				out["status"] = "ok"
				for _, v := range deps {
					if v != "up" {
						status = 503
						out["status"] = "degraded"
					}
				}
				out["dependencies"] = deps
			}
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(out)
		})
	}
}
func ping(ctx context.Context, url string) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}
	client := http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	res.Body.Close()
	return res.StatusCode == 200
}
