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
	for _, version := range []string{"001_async_grading", "002_teacher_review", "003_unified_subjects"} {
		var count int
		if err = conn.QueryRowContext(contextBackground(), "SELECT COUNT(*) FROM grade_migrations WHERE version=?", version).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if version == "002_teacher_review" {
			// Each column is checked separately so interrupted MySQL DDL can resume safely.
			for _, column := range []struct{ table, name, definition string }{
				{"grade_assignments", "subject", "VARCHAR(16) NOT NULL DEFAULT 'math'"},
				{"grade_results", "ai_correct", "BOOLEAN NULL"},
			} {
				if err = conn.QueryRowContext(contextBackground(), `SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND COLUMN_NAME=?`, column.table, column.name).Scan(&count); err != nil {
					return err
				}
				if count == 0 {
					if _, err = conn.ExecContext(contextBackground(), "ALTER TABLE "+column.table+" ADD COLUMN "+column.name+" "+column.definition); err != nil {
						return err
					}
				}
			}
		}
		data, err := migrations.ReadFile("migrations/" + version + ".sql")
		if err != nil {
			return err
		}
		for _, query := range strings.Split(string(data), ";") {
			if strings.TrimSpace(query) == "" {
				continue
			}
			if _, err = conn.ExecContext(contextBackground(), query); err != nil {
				return fmt.Errorf("grading migration %s: %w", version, err)
			}
		}
		if _, err = conn.ExecContext(contextBackground(), "INSERT INTO grade_migrations(version) VALUES(?)", version); err != nil {
			return err
		}
	}
	return nil
}
