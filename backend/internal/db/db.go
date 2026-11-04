// Package db 负责数据库连接（就绪重试）、建表、幂等种子与带班级过滤的查询。
// 隔离地基（design.md D3）：业务表 class_id 一律 NOT NULL 并建索引。
package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"

	"campusclaw/backend/internal/config"
)

// Open 创建连接池（不立即连通）。
func Open(cfg *config.Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		cfg.MySQLUser, cfg.MySQLPassword, cfg.MySQLHost, cfg.MySQLPort, cfg.MySQLDatabase)
	d, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(10)
	d.SetConnMaxLifetime(5 * time.Minute)
	return d, nil
}

// WaitReady 重试等待数据库可连接（design.md D4：depends_on 只保证启动顺序，不保证可连）。
func WaitReady(d *sql.DB) error {
	var err error
	for i := 0; i < 60; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = d.PingContext(ctx)
		cancel()
		if err == nil {
			return nil
		}
		log.Printf("等待数据库就绪（第 %d 次）：%v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("数据库在重试后仍不可连接：%w", err)
}

// Ready 供请求路径快速探活：数据库不可用时业务接口返回 503（design.md D9）。
func Ready(d *sql.DB) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return d.PingContext(ctx) == nil
}

// Migrate 建表（幂等）。作业/助手/技能表按规约预建，不验收业务功能。
func Migrate(d *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS material_seed_suppressions (class_id BIGINT NOT NULL,title VARCHAR(255) NOT NULL,PRIMARY KEY(class_id,title))`,
		`CREATE TABLE IF NOT EXISTS classes (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(64) NOT NULL UNIQUE
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			username VARCHAR(64) NOT NULL UNIQUE,
			password_hash VARCHAR(128) NOT NULL,
			role VARCHAR(16) NOT NULL,
			class_id BIGINT NOT NULL,
			INDEX idx_users_class (class_id),
			FOREIGN KEY (class_id) REFERENCES classes(id)
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id CHAR(64) PRIMARY KEY,
			user_id BIGINT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP NOT NULL,
			INDEX idx_sessions_user (user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS materials (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			class_id BIGINT NOT NULL,
			title VARCHAR(255) NOT NULL,
			file_path VARCHAR(512) NOT NULL,
			uploaded_by BIGINT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_materials_class (class_id),
			FOREIGN KEY (class_id) REFERENCES classes(id)
		)`,
		`CREATE TABLE IF NOT EXISTS knowledge_entries (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			material_id BIGINT NOT NULL,
			class_id BIGINT NOT NULL,
			body_text MEDIUMTEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_knowledge_class (class_id),
			INDEX idx_knowledge_material (material_id),
			FOREIGN KEY (class_id) REFERENCES classes(id)
		)`,
		// 切片表。正文与字符区间留 MySQL；向量主键与本表 id 一致（design.md D1）。
		// 关键字检索依赖 ngram 全文索引（中文必须 ngram，token 长度 2）。
		`CREATE TABLE IF NOT EXISTS knowledge_chunks (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			knowledge_entry_id BIGINT NOT NULL,
			material_id BIGINT NOT NULL,
			class_id BIGINT NOT NULL,
			chunk_index INT NOT NULL,
			chunk_text MEDIUMTEXT NOT NULL,
			char_start INT NOT NULL,
			char_end INT NOT NULL,
			index_status VARCHAR(16) NOT NULL DEFAULT 'pending',
			strategy VARCHAR(16) NOT NULL DEFAULT 'auto',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_chunks_class (class_id),
			INDEX idx_chunks_material (material_id),
			FULLTEXT KEY ft_chunk_text (chunk_text) WITH PARSER ngram
		)`,
		`CREATE TABLE IF NOT EXISTS assignments (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			class_id BIGINT NOT NULL,
			title VARCHAR(255) NOT NULL DEFAULT '',
			INDEX idx_assignments_class (class_id)
		)`,
		`CREATE TABLE IF NOT EXISTS assistants (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(64) NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS skills (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(64) NOT NULL DEFAULT ''
		)`,
	}
	for _, s := range stmts {
		if _, err := d.Exec(s); err != nil {
			return fmt.Errorf("建表失败：%w", err)
		}
	}
	var n int
	if err := d.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='materials' AND column_name='version'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := d.Exec(`ALTER TABLE materials ADD COLUMN version BIGINT NOT NULL DEFAULT 1`); err != nil {
			return err
		}
	}
	return nil
}

// Seed 幂等写入预置数据（先查后插）：重复启动不复制数据、不覆盖已上传内容。
// 口令只来自环境变量，以 bcrypt 哈希入库。
func Seed(d *sql.DB, cfg *config.Config) error {
	classA, err := ensureClass(d, "班级 A")
	if err != nil {
		return err
	}
	classB, err := ensureClass(d, "班级 B")
	if err != nil {
		return err
	}
	teacherA, err := ensureUser(d, "teacher_a", cfg.SeedTeacherPassword, "teacher", classA)
	if err != nil {
		return err
	}
	if _, err := ensureUser(d, "student_a1", cfg.SeedStudentPassword, "student", classA); err != nil {
		return err
	}
	if _, err := ensureUser(d, "student_b1", cfg.SeedStudentPassword, "student", classB); err != nil {
		return err
	}
	// 两班各一条标题可区分的种子材料 + 知识库正文（跨班失败路径的判定依据）。
	if err := ensureSeedMaterial(d, classA, teacherA, "A 班种子讲义：教研材料示例",
		"# A 班种子讲义\n\n这是班级 A 的预置教研材料正文，用于验收本班可见与跨班不可见。"); err != nil {
		return err
	}
	if err := ensureSeedMaterial(d, classB, teacherA, "B 班种子讲义：教研材料示例",
		"# B 班种子讲义\n\n这是班级 B 的预置教研材料正文，A 班用户在任何路径下都不应看到这条内容。"); err != nil {
		return err
	}
	return nil
}

func ensureClass(d *sql.DB, name string) (int64, error) {
	var id int64
	err := d.QueryRow(`SELECT id FROM classes WHERE name = ?`, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	res, err := d.Exec(`INSERT INTO classes (name) VALUES (?)`, name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func ensureUser(d *sql.DB, username, password, role string, classID int64) (int64, error) {
	var id int64
	err := d.QueryRow(`SELECT id FROM users WHERE username = ?`, username).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}
	res, err := d.Exec(`INSERT INTO users (username, password_hash, role, class_id) VALUES (?, ?, ?, ?)`,
		username, string(hash), role, classID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func ensureSeedMaterial(d *sql.DB, classID, uploader int64, title, body string) error {
	var suppressed int
	if err := d.QueryRow(`SELECT COUNT(*) FROM material_seed_suppressions WHERE class_id=? AND title=?`, classID, title).Scan(&suppressed); err != nil {
		return err
	}
	if suppressed > 0 {
		return nil
	}
	var id int64
	err := d.QueryRow(`SELECT id FROM materials WHERE class_id = ? AND title = ?`, classID, title).Scan(&id)
	if err == nil {
		return nil // 已存在：幂等跳过，不覆盖
	}
	if err != sql.ErrNoRows {
		return err
	}
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO materials (class_id, title, file_path, uploaded_by) VALUES (?, ?, ?, ?)`,
		classID, title, "", uploader)
	if err != nil {
		return err
	}
	mid, err := res.LastInsertId()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO knowledge_entries (material_id, class_id, body_text) VALUES (?, ?, ?)`,
		mid, classID, body); err != nil {
		return err
	}
	return tx.Commit()
}
