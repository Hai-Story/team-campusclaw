package grading

import (
	"database/sql"
	"embed"
	"fmt"
	"strings"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate serializes schema changes. Version is recorded only after every DDL succeeds.
func Migrate(db *sql.DB) error {
	conn, err := db.Conn(contextBackground())
	if err != nil {
		return err
	}
	defer conn.Close()
	var locked int
	if err = conn.QueryRowContext(contextBackground(), "SELECT GET_LOCK('campusclaw_grade_migration',30)").Scan(&locked); err != nil || locked != 1 {
		return fmt.Errorf("migration lock unavailable")
	}
	defer conn.ExecContext(contextBackground(), "SELECT RELEASE_LOCK('campusclaw_grade_migration')")
	if _, err = conn.ExecContext(contextBackground(), `CREATE TABLE IF NOT EXISTS grade_migrations(version VARCHAR(64) PRIMARY KEY,applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6))`); err != nil {
		return err
	}
	var count int
	if err = conn.QueryRowContext(contextBackground(), "SELECT COUNT(*) FROM grade_migrations WHERE version='001_async_grading'").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	data, err := migrations.ReadFile("migrations/001_async_grading.sql")
	if err != nil {
		return err
	}
	for _, query := range strings.Split(string(data), ";") {
		if strings.TrimSpace(query) == "" {
			continue
		}
		if _, err = conn.ExecContext(contextBackground(), query); err != nil {
			return fmt.Errorf("grading migration: %w", err)
		}
	}
	_, err = conn.ExecContext(contextBackground(), "INSERT INTO grade_migrations(version) VALUES('001_async_grading')")
	return err
}
