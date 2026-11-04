// Package materials 实现材料列表、详情、文件下载与教师上传。
// 隔离约束（design.md D3/D6/D7、spec R3/R4/R5）：
//   - 列表强制 WHERE class_id = 会话班级；请求参数中的 class_id 一律忽略。
//   - 按 ID 访问先取行再核对班级；跨班与不存在返回完全同形的 404。
//   - 上传：白名单扩展名、超限 413 提前拒绝、服务端生成存储名、双表同事务、失败无残留。
package materials

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"campusclaw/backend/internal/auth"
	"campusclaw/backend/internal/chunking"
	"campusclaw/backend/internal/config"
	"campusclaw/backend/internal/knowledge"
	"campusclaw/backend/internal/retrieval"
)

// Service 持有材料相关依赖。
type Service struct {
	DB      *sql.DB
	Cfg     *config.Config
	indexer *retrieval.Indexer // 上传成功后追加切片索引管线（）
}

func NewService(d *sql.DB, cfg *config.Config) *Service {
	return &Service{DB: d, Cfg: cfg}
}

// SetIndexer 注入索引器（main 组装依赖后调用）。
func (s *Service) SetIndexer(ix *retrieval.Indexer) {
	s.indexer = ix
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// notFound 是跨班访问与"记录不存在"的统一对外响应（spec R3 同形 404）。
func notFound(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "材料不存在"})
}

// Summary 是列表条目（绝不包含 file_path 等存储信息）。
type Summary struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	ClassName string    `json:"class_name"`
	CreatedAt time.Time `json:"created_at"`
}

// List 处理 GET /api/materials：只按会话班级过滤，支持本班关键词搜索。
// @api {"method":"GET","path":"/api/materials","summary":"列出并搜索本班材料","auth":"login","status":200,"request":null,"response":{"type":"array","items":{"$ref":"#/components/schemas/Material"}},"errors":{"401":"未登录","503":"服务暂时不可用"},"parameters":[{"name":"q","in":"query","schema":{"type":"string"}}]}
func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromContext(r.Context())
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	// 注意：query/header/表单中的 class_id 一律忽略，租户条件只取自会话。
	var rows *sql.Rows
	var err error
	if q == "" {
		rows, err = s.DB.Query(`
			SELECT m.id, m.title, c.name, m.created_at
			FROM materials m JOIN classes c ON c.id = m.class_id
			WHERE m.class_id = ?
			ORDER BY m.created_at DESC`, u.ClassID)
	} else {
		like := "%" + q + "%"
		rows, err = s.DB.Query(`
			SELECT m.id, m.title, c.name, m.created_at
			FROM materials m
			JOIN classes c ON c.id = m.class_id
			LEFT JOIN knowledge_entries k ON k.material_id = m.id
			WHERE m.class_id = ? AND (m.title LIKE ? OR k.body_text LIKE ?)
			GROUP BY m.id, m.title, c.name, m.created_at
			ORDER BY m.created_at DESC`, u.ClassID, like, like)
	}
	if err != nil {
		log.Printf("列表查询失败：%v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "服务暂时不可用"})
		return
	}
	defer rows.Close()
	out := []Summary{}
	for rows.Next() {
		var m Summary
		if err := rows.Scan(&m.ID, &m.Title, &m.ClassName, &m.CreatedAt); err != nil {
			log.Printf("列表扫描失败：%v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "服务器错误"})
			return
		}
		out = append(out, m)
	}
	writeJSON(w, http.StatusOK, out)
}

// materialRow 是"先取行再校验"（fetch-then-check）的中间结构。
type materialRow struct {
	version   int64
	id        int64
	classID   int64
	className string
	title     string
	filePath  string
	createdAt time.Time
}

// fetchByID 按 ID 取行，故意不在 SQL 里过滤班级（design.md D3 对象路径）。
func (s *Service) fetchByID(id int64) (*materialRow, error) {
	var m materialRow
	err := s.DB.QueryRow(`
		SELECT m.id, m.class_id, c.name, m.title, m.file_path, m.created_at,m.version
		FROM materials m JOIN classes c ON c.id = m.class_id
		WHERE m.id = ?`, id).
		Scan(&m.id, &m.classID, &m.className, &m.title, &m.filePath, &m.createdAt, &m.version)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// fetchAuthorized 取行并核对班级；跨班时写服务端日志但对外与"不存在"同形。
func (s *Service) fetchAuthorized(w http.ResponseWriter, r *http.Request) *materialRow {
	u, _ := auth.FromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		notFound(w)
		return nil
	}
	m, err := s.fetchByID(id)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("按 ID 查询失败：%v", err)
		}
		notFound(w)
		return nil
	}
	if m.classID != u.ClassID {
		log.Printf("安全事件：用户 %s（班级 %d）尝试跨班访问材料 %d（班级 %d）",
			u.Username, u.ClassID, m.id, m.classID)
		notFound(w)
		return nil
	}
	return m
}

// Detail 处理 GET /api/materials/{id}：返回元数据与知识库正文，不含磁盘路径。
// @api {"method":"GET","path":"/api/materials/{id}","summary":"读取材料正文","auth":"login","status":200,"request":null,"response":{"type":"object","properties":{"id":{"type":"integer","format":"int64"},"title":{"type":"string"},"body":{"type":"string"},"class_name":{"type":"string"},"created_at":{"type":"string"},"version":{"type":"integer","minimum":1}}},"errors":{"401":"未登录","404":"材料不存在（含跨班）","503":"服务暂时不可用"}}
func (s *Service) Detail(w http.ResponseWriter, r *http.Request) {
	m := s.fetchAuthorized(w, r)
	if m == nil {
		return
	}
	var body string
	err := s.DB.QueryRow(`SELECT body_text FROM knowledge_entries WHERE material_id = ? ORDER BY id LIMIT 1`, m.id).Scan(&body)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("知识库查询失败：%v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "服务暂时不可用"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":         m.id,
		"title":      m.title,
		"class_name": m.className,
		"created_at": m.createdAt,
		"body":       body,
		"version":    m.version,
	})
}

// File 处理 GET /api/materials/{id}/file：鉴权后由 Go 读盘回传，不静态暴露上传目录。
// @api {"method":"GET","path":"/api/materials/{id}/file","summary":"鉴权下载原文件","auth":"login","status":200,"request":null,"response":{"type":"string"},"errors":{"401":"未登录","404":"材料不存在或文件不可读（含跨班）","503":"服务暂时不可用"},"media":"text/plain"}
func (s *Service) File(w http.ResponseWriter, r *http.Request) {
	m := s.fetchAuthorized(w, r)
	if m == nil {
		return
	}
	if m.filePath == "" {
		notFound(w) // 种子材料无落盘文件，对外同样不暴露存储细节
		return
	}
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		log.Printf("读取文件失败 %q：%v", m.filePath, err)
		notFound(w)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="material-`+strconv.FormatInt(m.id, 10)+filepath.Ext(m.filePath)+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

var allowedExt = map[string]bool{".txt": true, ".md": true}

// Upload 处理 POST /api/materials（RequireTeacher 之后）。
// @api {"method":"POST","path":"/api/materials","summary":"教师上传非空UTF-8 .txt/.md","auth":"teacher","status":201,"request":{"type":"object","properties":{"file":{"type":"string","format":"binary"},"title":{"type":"string"}},"required":["file"]},"response":{"type":"object","properties":{"id":{"type":"integer","format":"int64"},"title":{"type":"string"}}},"errors":{"400":"非法扩展名/编码/内容","401":"未登录","403":"仅教师可写","413":"超过上传上限","500":"入库失败","503":"服务暂时不可用"},"requestMedia":"multipart/form-data"}
func (s *Service) Upload(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromContext(r.Context())

	// 超限在读取完整请求体之前拒绝（design.md D6）：MaxBytesReader 提前截断。
	r.Body = http.MaxBytesReader(w, r.Body, s.Cfg.MaxUploadBytes)
	if err := r.ParseMultipartForm(s.Cfg.MaxUploadBytes); err != nil {
		var mbErr *http.MaxBytesError
		if errors.As(err, &mbErr) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "文件超过大小上限"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式非法"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少上传文件"})
		return
	}
	defer file.Close()

	// 扩展名白名单（.md.exe 这类双扩展名同样被拒）。
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedExt[ext] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "仅支持 .txt / .md 文件"})
		return
	}
	data, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "读取文件失败"})
		return
	}
	body, err := knowledge.ParseText(data)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "文件内容不是非空的 UTF-8 文本"})
		return
	}

	// 标题：表单 title 优先，否则用客户端文件名（仅作展示，不参与路径构造）。
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = filepath.Base(header.Filename)
	}

	// 存储名由服务端生成，杜绝路径穿越。
	randBytes := make([]byte, 16)
	if _, err := rand.Read(randBytes); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "服务器错误"})
		return
	}
	storedName := hex.EncodeToString(randBytes) + ext
	storedPath := filepath.Join(s.Cfg.UploadDir, storedName)
	if err := os.MkdirAll(s.Cfg.UploadDir, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "服务器错误"})
		return
	}
	if err := os.WriteFile(storedPath, data, 0o644); err != nil {
		log.Printf("写盘失败：%v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "服务器错误"})
		return
	}

	// 双表同事务；任一步失败整体回滚并删除已写文件，不留孤儿（spec R4）。
	// 班级归属只取自会话，表单中的任何班级字段都被忽略（spec R3）。
	tx, err := s.DB.Begin()
	if err != nil {
		os.Remove(storedPath)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "服务暂时不可用"})
		return
	}
	cleanup := func() {
		tx.Rollback()
		os.Remove(storedPath)
	}
	res, err := tx.Exec(`INSERT INTO materials (class_id, title, file_path, uploaded_by) VALUES (?, ?, ?, ?)`,
		u.ClassID, title, storedPath, u.ID)
	if err != nil {
		log.Printf("写入材料失败：%v", err)
		cleanup()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "入库失败"})
		return
	}
	mid, err := res.LastInsertId()
	if err != nil {
		cleanup()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "入库失败"})
		return
	}
	res2, err := tx.Exec(`INSERT INTO knowledge_entries (material_id, class_id, body_text) VALUES (?, ?, ?)`,
		mid, u.ClassID, body)
	if err != nil {
		log.Printf("写入知识库失败：%v", err)
		cleanup()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "入库失败"})
		return
	}
	entryID, err := res2.LastInsertId()
	if err != nil {
		cleanup()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "入库失败"})
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("事务提交失败：%v", err)
		cleanup()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "入库失败"})
		return
	}
	// 上传成功后按 auto 策略切分、嵌入、写入 Qdrant（spec 切片入库管线）。
	// 嵌入失败不丢原文——材料记录保留，对应切片标 failed，上传仍返回成功。
	if s.indexer != nil {
		if _, err := s.indexer.IndexMaterial(r.Context(), mid, entryID, u.ClassID, body, "auto", chunking.Options{}); err != nil {
			log.Printf("上传后索引失败（material %d）：%v", mid, err)
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": mid, "title": title})
}
