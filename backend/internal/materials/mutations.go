package materials

import (
	"bytes"
	"campusclaw/backend/internal/auth"
	"campusclaw/backend/internal/chunking"
	"campusclaw/backend/internal/knowledge"
	"context"
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
	"strings"
	"unicode/utf8"
)

type mutationFailure struct {
	status  int
	message string
}

func (e mutationFailure) Error() string { return e.message }
func (s *Service) guardedMutation(ctx context.Context, id int64, fn func() error) error {
	if s.indexer != nil {
		return s.indexer.WithMaterial(ctx, id, fn)
	}
	return fn()
}
func mutationResponse(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	var failure mutationFailure
	if errors.As(err, &failure) {
		writeJSON(w, failure.status, map[string]string{"error": failure.message})
	} else {
		log.Printf("资料维护失败：%v", err)
		writeJSON(w, 503, map[string]string{"error": "资料维护暂时不可用"})
	}
}
func (s *Service) removeOwnedFile(name string) {
	if name == "" {
		return
	}
	root, err := filepath.Abs(s.Cfg.UploadDir)
	if err != nil {
		return
	}
	file, err := filepath.Abs(name)
	if err != nil {
		return
	}
	rel, err := filepath.Rel(root, file)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return
	}
	if err = os.Remove(file); err != nil && !os.IsNotExist(err) {
		log.Printf("旧资料文件清理失败：%v", err)
	}
}
func (s *Service) writeRevision(body, old string) (string, error) {
	ext := strings.ToLower(filepath.Ext(old))
	if !allowedExt[ext] {
		ext = ".md"
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.Cfg.UploadDir, 0755); err != nil {
		return "", err
	}
	file := filepath.Join(s.Cfg.UploadDir, hex.EncodeToString(b)+ext)
	if err := os.WriteFile(file, []byte(body), 0644); err != nil {
		return "", err
	}
	return file, nil
}
func (s *Service) readMutation(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, s.Cfg.MaxUploadBytes*6+8192)
	data, err := io.ReadAll(r.Body)
	if err != nil || !utf8.Valid(data) {
		writeJSON(w, 400, map[string]string{"error": "请求必须是有效UTF-8 JSON"})
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(v); err != nil {
		writeJSON(w, 400, map[string]string{"error": "请求必须为合法JSON"})
		return false
	}
	return true
}
func lockedMaterial(ctx context.Context, tx *sql.Tx, id, class int64) (title, file string, version int64, err error) {
	err = tx.QueryRowContext(ctx, `SELECT title,file_path,version FROM materials WHERE id=? AND class_id=? FOR UPDATE`, id, class).Scan(&title, &file, &version)
	if errors.Is(err, sql.ErrNoRows) {
		err = mutationFailure{404, "材料不存在"}
	}
	return
}

// @api {"method":"PUT","path":"/api/materials/{id}","summary":"教师修改本班资料标题和正文并重建检索","auth":"teacher","status":200,"request":{"type":"object","required":["title","body","version"],"properties":{"title":{"type":"string","minLength":1,"maxLength":255},"body":{"type":"string","minLength":1},"version":{"type":"integer","minimum":1}}},"response":{"type":"object","properties":{"id":{"type":"integer"},"title":{"type":"string"},"version":{"type":"integer"},"index_status":{"type":"string","enum":["ready","failed","unavailable"]}}},"errors":{"400":"标题或正文非法","401":"未登录","403":"仅教师可修改","404":"材料不存在（含跨班）","409":"资料版本已更新，请重新加载","413":"正文超过大小上限","503":"资料维护暂时不可用"},"description":"原子保存当前标题、正文及下载文件，删除旧切片后重建。嵌入失败保留新正文并返回index_status=failed。仅本班教师可写。"}
func (s *Service) Update(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromContext(r.Context())
	m := s.fetchAuthorized(w, r)
	if m == nil {
		return
	}
	var in struct {
		Title   string `json:"title"`
		Body    string `json:"body"`
		Version int64  `json:"version"`
	}
	if !s.readMutation(w, r, &in) {
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" || !utf8.ValidString(in.Title) || strings.ContainsRune(in.Title, 0) || len([]rune(in.Title)) > 255 || in.Version < 1 {
		writeJSON(w, 400, map[string]string{"error": "标题须为1至255字，版本必须有效"})
		return
	}
	if int64(len(in.Body)) > s.Cfg.MaxUploadBytes {
		writeJSON(w, 413, map[string]string{"error": "正文超过大小上限"})
		return
	}
	text, err := knowledge.ParseText([]byte(in.Body))
	if err != nil || strings.ContainsRune(in.Body, 0) {
		writeJSON(w, 400, map[string]string{"error": "正文必须是非空UTF-8文本"})
		return
	}
	indexStatus := "unavailable"
	err = s.guardedMutation(r.Context(), m.id, func() error {
		tx, err := s.DB.BeginTx(r.Context(), nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		oldTitle, oldFile, version, err := lockedMaterial(r.Context(), tx, m.id, u.ClassID)
		if err != nil {
			return err
		}
		if version != in.Version {
			return mutationFailure{409, "资料版本已更新，请重新加载"}
		}
		file, err := s.writeRevision(text, oldFile)
		if err != nil {
			return err
		}
		committed := false
		defer func() {
			if !committed {
				s.removeOwnedFile(file)
			}
		}()
		if _, err = tx.ExecContext(r.Context(), `UPDATE materials SET title=?,file_path=?,version=version+1 WHERE id=?`, in.Title, file, m.id); err != nil {
			return err
		}
		var entry int64
		err = tx.QueryRowContext(r.Context(), `SELECT id FROM knowledge_entries WHERE material_id=? ORDER BY id LIMIT 1`, m.id).Scan(&entry)
		if err == sql.ErrNoRows {
			_, err = tx.ExecContext(r.Context(), `INSERT INTO knowledge_entries(material_id,class_id,body_text) VALUES(?,?,?)`, m.id, u.ClassID, text)
		} else if err == nil {
			_, err = tx.ExecContext(r.Context(), `UPDATE knowledge_entries SET body_text=? WHERE material_id=? AND class_id=?`, text, m.id, u.ClassID)
		}
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(r.Context(), `DELETE FROM knowledge_chunks WHERE material_id=?`, m.id); err != nil {
			return err
		}
		if oldTitle != in.Title {
			if _, err = tx.ExecContext(r.Context(), `INSERT IGNORE INTO material_seed_suppressions(class_id,title) VALUES(?,?)`, u.ClassID, oldTitle); err != nil {
				return err
			}
		}
		if err = tx.Commit(); err != nil {
			return err
		}
		committed = true
		s.removeOwnedFile(oldFile)
		if s.indexer != nil {
			indexStatus = "failed"
			if _, e := s.indexer.IndexCurrentLocked(r.Context(), m.id, "auto", chunking.Options{}); e == nil {
				var total, failed int
				s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*),COALESCE(SUM(index_status<>'ready'),0) FROM knowledge_chunks WHERE material_id=?`, m.id).Scan(&total, &failed)
				if total > 0 && failed == 0 {
					indexStatus = "ready"
				}
			}
		}
		return nil
	})
	if err != nil {
		mutationResponse(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"id": m.id, "title": in.Title, "version": in.Version + 1, "index_status": indexStatus})
}

// @api {"method":"DELETE","path":"/api/materials/{id}","summary":"教师确认删除本班资料及知识库、绑定","auth":"teacher","status":204,"request":{"type":"object","required":["version"],"properties":{"version":{"type":"integer","minimum":1}}},"response":null,"errors":{"400":"版本非法","401":"未登录","403":"仅教师可删除","404":"材料不存在（含跨班）","409":"资料版本已更新，请重新加载","503":"资料维护暂时不可用"},"description":"删除正文、切片、智能体绑定和材料记录，清理受控下载文件及远端向量；向量故障时SQL回表拒绝孤立点。二次确认由教师界面执行，种子材料删除后不复活。"}
func (s *Service) Delete(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromContext(r.Context())
	m := s.fetchAuthorized(w, r)
	if m == nil {
		return
	}
	var in struct {
		Version int64 `json:"version"`
	}
	if !s.readMutation(w, r, &in) {
		return
	}
	if in.Version < 1 {
		writeJSON(w, 400, map[string]string{"error": "版本必须有效"})
		return
	}
	err := s.guardedMutation(r.Context(), m.id, func() error {
		tx, err := s.DB.BeginTx(r.Context(), nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		title, file, version, err := lockedMaterial(r.Context(), tx, m.id, u.ClassID)
		if err != nil {
			return err
		}
		if version != in.Version {
			return mutationFailure{409, "资料版本已更新，请重新加载"}
		}
		for _, q := range []string{`DELETE FROM tutor_agent_materials WHERE material_id=?`, `DELETE FROM knowledge_chunks WHERE material_id=?`, `DELETE FROM knowledge_entries WHERE material_id=?`, `DELETE FROM materials WHERE id=?`} {
			if _, err = tx.ExecContext(r.Context(), q, m.id); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT IGNORE INTO material_seed_suppressions(class_id,title) VALUES(?,?)`, u.ClassID, title); err != nil {
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
		s.removeOwnedFile(file)
		if s.indexer != nil && s.indexer.Qdr != nil {
			if e := s.indexer.Qdr.DeleteByMaterial(r.Context(), m.id); e != nil {
				log.Printf("资料%d向量清理失败：%v", m.id, e)
			}
		}
		return nil
	})
	if err != nil {
		mutationResponse(w, err)
		return
	}
	w.WriteHeader(204)
}
