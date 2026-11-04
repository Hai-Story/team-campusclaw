// Package auth 实现登录、登出、Token 校验中间件与登录限流。
// 关键约束（design.md D1/D2/D5、spec R1/R2）：
//   - 会话存服务端表，Token 为不透明随机串经 Authorization: Bearer 头携带；库中存 Token 的 HMAC（SESSION_SECRET）。
//   - 角色与班级每次请求从用户表读取，不采信客户端声明。
//   - 登录成功换发新 Token（旧 Token 一并作废）；登出删会话行立即失效。
//   - 账号不存在 / 口令错误 / 被限流三类失败响应完全同形。
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"campusclaw/backend/internal/config"
)

// bearerPrefix 是受保护 API 读取凭据的请求头前缀（spec R1）。
const bearerPrefix = "Bearer "

// tokenKey 把不透明 Token 映射为库中存储的 HMAC 值：
// 即使数据库泄露，攻击者也无法直接拿到可用的 Token。
func (s *Service) tokenKey(id string) string {
	mac := hmac.New(sha256.New, []byte(s.Cfg.SessionSecret))
	mac.Write([]byte(id))
	return hex.EncodeToString(mac.Sum(nil))
}

// User 是每次请求从数据库读出的授权上下文（用户、角色、班级三项齐备）。
type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	ClassID   int64  `json:"class_id"`
	ClassName string `json:"class_name"`
}

type ctxKey struct{}

// FromContext 取出中间件注入的用户；未登录时 ok=false。
func FromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(ctxKey{}).(User)
	return u, ok
}

// Service 持有认证所需的依赖。
type Service struct {
	DB      *sql.DB
	Cfg     *config.Config
	limiter *rateLimiter
}

func NewService(d *sql.DB, cfg *config.Config) *Service {
	return &Service{DB: d, Cfg: cfg, limiter: newRateLimiter(cfg.LoginFailLimit, cfg.LoginFailWindow)}
}

// newToken 生成 256-bit 随机不透明 Token（不含角色、班级等可读信息）。
func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// bearerToken 从 Authorization 头解析 Bearer Token；缺失或格式不符返回空串，
// 与「无效 Token」在中间件中一律 401 同形（不泄露校验细节，design.md D2）。
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, bearerPrefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, bearerPrefix))
}

// --- 登录 / 登出 / 身份 ---

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// dummyHash 用于"账号不存在"时的等价耗时比较，保证三类失败响应同形（spec R1）。
var dummyHash = []byte("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy")

// Login 处理 POST /api/login。
// @api {"method":"POST","path":"/api/login","summary":"登录签发不透明Token","auth":"public","status":200,"request":{"type":"object","properties":{"username":{"type":"string"},"password":{"type":"string","format":"password"}},"required":["username","password"]},"response":{"type":"object","properties":{"token":{"type":"string"},"user":{"$ref":"#/components/schemas/User"}}},"errors":{"401":"用户名或密码错误（含限流）","503":"服务暂时不可用"}}
func (s *Service) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "用户名或密码错误"})
		return
	}
	key := req.Username + "|" + clientIP(r)
	// 限流命中：即使口令正确也拒绝，响应与口令错误完全同形（不暴露锁定状态）。
	limited := s.limiter.limited(key)

	var (
		id       int64
		hash     string
		role     string
		classID  int64
		username string
	)
	row := s.DB.QueryRow(`SELECT id, username, password_hash, role, class_id FROM users WHERE username = ?`, req.Username)
	err := row.Scan(&id, &username, &hash, &role, &classID)

	checkAgainst := dummyHash
	if err == nil {
		checkAgainst = []byte(hash)
	} else if err != sql.ErrNoRows {
		log.Printf("登录查询失败：%v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "服务暂时不可用"})
		return
	}
	// 无论账号是否存在都执行一次 bcrypt 比较，消除耗时差异。
	pwOK := bcrypt.CompareHashAndPassword(checkAgainst, []byte(req.Password)) == nil && err == nil

	if limited || !pwOK {
		if !pwOK {
			s.limiter.fail(key)
		}
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "用户名或密码错误"})
		return
	}
	s.limiter.reset(key)

	// 防会话固定：换发新 Token；若请求带旧 Token（Authorization 头）则一并作废。
	if old := bearerToken(r); old != "" {
		_, _ = s.DB.Exec(`DELETE FROM sessions WHERE id = ?`, s.tokenKey(old))
	}
	token, err := newToken()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "服务器错误"})
		return
	}
	expires := time.Now().Add(s.Cfg.SessionTTL)
	if _, err := s.DB.Exec(`INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)`,
		s.tokenKey(token), id, expires); err != nil {
		log.Printf("写入会话失败：%v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "服务暂时不可用"})
		return
	}
	// 登录成功返回 Token 与授权上下文（user 与 GET /api/me 同构，design.md D4）；
	// 不再 Set-Cookie，凭据改由前端存 localStorage 并经 Authorization 头携带。
	var className string
	if err := s.DB.QueryRow(`SELECT name FROM classes WHERE id = ?`, classID).Scan(&className); err != nil {
		className = ""
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user":  User{ID: id, Username: username, Role: role, ClassID: classID, ClassName: className},
	})
}

// Logout 处理 POST /api/logout：删除服务端会话行，旧 Token 立即失效。
// @api {"method":"POST","path":"/api/logout","summary":"撤销当前Token；无Token亦幂等成功","auth":"public","status":200,"request":null,"response":{"type":"object","properties":{"ok":{"type":"boolean"}}},"errors":{"503":"服务暂时不可用"},"description":"可携带Bearer撤销服务端会话；该接口不要求已有有效会话。"}
func (s *Service) Logout(w http.ResponseWriter, r *http.Request) {
	if t := bearerToken(r); t != "" {
		_, _ = s.DB.Exec(`DELETE FROM sessions WHERE id = ?`, s.tokenKey(t))
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Me 处理 GET /api/me：返回用户、角色、班级三项，供前端决定上传入口。
// @api {"method":"GET","path":"/api/me","summary":"读取身份、角色与班级","auth":"login","status":200,"request":null,"response":{"$ref":"#/components/schemas/User"},"errors":{"401":"未登录","503":"服务暂时不可用"}}
func (s *Service) Me(w http.ResponseWriter, r *http.Request) {
	u, _ := FromContext(r.Context())
	writeJSON(w, http.StatusOK, u)
}

// Middleware 校验 Authorization: Bearer Token 并把 User 注入上下文。
// 缺头、格式错误、无效或过期 Token 一律 401，响应体不含任何业务数据（spec R1 未登录 Scenario）。
func (s *Service) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t := bearerToken(r)
		if t == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}
		var u User
		row := s.DB.QueryRow(`
			SELECT u.id, u.username, u.role, u.class_id, c.name
			FROM sessions s
			JOIN users u ON u.id = s.user_id
			JOIN classes c ON c.id = u.class_id
			WHERE s.id = ? AND s.expires_at > NOW()`, s.tokenKey(t))
		if err := row.Scan(&u.ID, &u.Username, &u.Role, &u.ClassID, &u.ClassName); err != nil {
			if err != sql.ErrNoRows {
				log.Printf("会话查询失败：%v", err)
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "服务暂时不可用"})
				return
			}
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
	}
}

// RequireTeacher 在会话之上再加角色判断；学生写操作一律 403（spec R2）。
func RequireTeacher(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := FromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}
		if u.Role != "teacher" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "仅教师可执行此操作"})
			return
		}
		next(w, r)
	}
}

// --- 登录失败限流（进程内存，故本迭代为单实例；design.md D8） ---

type failRecord struct {
	count   int
	resetAt time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	records map[string]*failRecord
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{limit: limit, window: window, records: map[string]*failRecord{}}
}

func (rl *rateLimiter) limited(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rec, ok := rl.records[key]
	if !ok || time.Now().After(rec.resetAt) {
		return false
	}
	return rec.count >= rl.limit
}

func (rl *rateLimiter) fail(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rec, ok := rl.records[key]
	now := time.Now()
	if !ok || now.After(rec.resetAt) {
		rl.records[key] = &failRecord{count: 1, resetAt: now.Add(rl.window)}
		return
	}
	rec.count++
}

func (rl *rateLimiter) reset(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.records, key)
}

// RequireStudent keeps the teacher workspace focused on configuration.
func RequireStudent(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := FromContext(r.Context())
		if !ok {
			writeJSON(w, 401, map[string]string{"error": "未登录"})
			return
		}
		if u.Role != "student" {
			writeJSON(w, 403, map[string]string{"error": "仅学生可使用智能体会话"})
			return
		}
		next(w, r)
	}
}
