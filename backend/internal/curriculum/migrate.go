package curriculum

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate only adds versioned structures; rolling back the application keeps data.
func Migrate(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var lock int
	if err = conn.QueryRowContext(ctx, "SELECT GET_LOCK('campusclaw_curriculum_migration',30)").Scan(&lock); err != nil || lock != 1 {
		return fmt.Errorf("curriculum migration lock unavailable")
	}
	defer conn.ExecContext(context.WithoutCancel(ctx), "SELECT RELEASE_LOCK('campusclaw_curriculum_migration')")
	if _, err = conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS curriculum_migrations(version VARCHAR(64) PRIMARY KEY,applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6))`); err != nil {
		return err
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		var count int
		if err = conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM curriculum_migrations WHERE version=?", entry.Name()).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		data, e := migrations.ReadFile("migrations/" + entry.Name())
		if e != nil {
			return e
		}
		if entry.Name() == "008_nine_subjects.sql" {
			if err = migrateSubjects(ctx, conn, entry.Name(), string(data)); err != nil {
				return err
			}
			continue
		}
		for _, q := range strings.Split(string(data), ";") {
			if strings.TrimSpace(q) != "" {
				if _, err = conn.ExecContext(ctx, q); err != nil {
					return fmt.Errorf("curriculum migration %s: %w", entry.Name(), err)
				}
			}
		}
		if _, err = conn.ExecContext(ctx, "INSERT INTO curriculum_migrations(version) VALUES(?)", entry.Name()); err != nil {
			return err
		}
	}
	return nil
}

// DML and the version marker commit together; failed classification cannot leave
// a raw transaction open on a pooled connection or partially move related rows.
func migrateSubjects(ctx context.Context, conn *sql.Conn, version, script string) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range strings.Split(script, ";") {
		q := strings.TrimSpace(statement)
		if q == "" || q == "START TRANSACTION" || q == "COMMIT" {
			continue
		}
		if _, err = tx.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("curriculum migration %s: %w", version, err)
		}
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO curriculum_migrations(version) VALUES(?)", version); err != nil {
		return err
	}
	return tx.Commit()
}
