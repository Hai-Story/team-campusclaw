package curriculum

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"campusclaw/backend/internal/auth"
	"github.com/go-sql-driver/mysql"
)

type Store struct{ DB *sql.DB }
type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

const pointColumns = `p.id,p.subject,p.name,p.chapter,p.description,p.version,p.created_at,p.updated_at,
 COALESCE((SELECT JSON_ARRAYAGG(material_id) FROM knowledge_point_materials WHERE point_id=p.id),JSON_ARRAY()),
 COALESCE((SELECT JSON_ARRAYAGG(assignment_id) FROM knowledge_point_assignments WHERE point_id=p.id),JSON_ARRAY()),
 EXISTS(SELECT 1 FROM knowledge_favorites f WHERE f.point_id=p.id AND f.student_id=?)`

type scanner interface{ Scan(...any) error }

func scanPoint(row scanner) (Point, error) {
	var p Point
	var materials, assignments []byte
	err := row.Scan(&p.ID, &p.Subject, &p.Name, &p.Chapter, &p.Description, &p.Version, &p.CreatedAt, &p.UpdatedAt, &materials, &assignments, &p.Favorite)
	if err != nil {
		return p, normalize(err)
	}
	if err = json.Unmarshal(materials, &p.MaterialIDs); err != nil {
		return p, err
	}
	if err = json.Unmarshal(assignments, &p.AssignmentIDs); err != nil {
		return p, err
	}
	return p, nil
}
func normalize(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	var m *mysql.MySQLError
	if errors.As(err, &m) && m.Number == 1062 {
		return ErrConflict
	}
	return err
}
func point(ctx context.Context, q queryer, u auth.User, id int64) (Point, error) {
	return scanPoint(q.QueryRowContext(ctx, `SELECT `+pointColumns+` FROM knowledge_points p WHERE p.id=? AND p.class_id=?`, u.ID, id, u.ClassID))
}
func (s *Store) Point(ctx context.Context, u auth.User, id int64) (Point, error) {
	return point(ctx, s.DB, u, id)
}
func (s *Store) Points(ctx context.Context, u auth.User, f PointFilter) ([]Point, error) {
	query := `SELECT ` + pointColumns + ` FROM knowledge_points p WHERE p.class_id=? AND p.subject=?`
	args := []any{u.ID, u.ClassID, f.Subject}
	if f.Q != "" {
		query += ` AND (p.name LIKE ? OR p.chapter LIKE ?)`
		args = append(args, "%"+f.Q+"%", "%"+f.Q+"%")
	}
	if f.Favorites {
		query += ` AND EXISTS(SELECT 1 FROM knowledge_favorites f WHERE f.point_id=p.id AND f.student_id=?)`
		args = append(args, u.ID)
	}
	query += ` ORDER BY p.id DESC LIMIT ? OFFSET ?`
	args = append(args, f.Size, int64(f.Page-1)*int64(f.Size))
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Point{}
	for rows.Next() {
		p, e := scanPoint(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Store) SavePoint(ctx context.Context, u auth.User, id int64, in PointInput) (Point, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Point{}, err
	}
	defer tx.Rollback()
	if err = graphScope(ctx, tx, u, in.Subject); err != nil {
		return Point{}, err
	}
	var oldSubject string
	if id > 0 {
		var version int
		if err = tx.QueryRowContext(ctx, `SELECT version,subject FROM knowledge_points WHERE id=? AND class_id=? FOR UPDATE`, id, u.ClassID).Scan(&version, &oldSubject); err != nil {
			return Point{}, normalize(err)
		}
		if version != *in.ExpectedVersion {
			return Point{}, ErrConflict
		}
	}

	if id == 0 || oldSubject != in.Subject {
		var count int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge_points WHERE class_id=? AND subject=?`, u.ClassID, in.Subject).Scan(&count); err != nil {
			return Point{}, err
		}
		if count >= 500 {
			return Point{}, ErrConflict
		}
		if id > 0 {
			if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge_graph_edges WHERE class_id=? AND (from_id=? OR to_id=?)`, u.ClassID, id, id).Scan(&count); err != nil {
				return Point{}, err
			}
			if count > 0 {
				return Point{}, ErrConflict
			}
		}
	}
	// All resource checks precede mutation and hold a shared lock until commit.
	for _, m := range in.MaterialIDs {
		var found int64
		if err = tx.QueryRowContext(ctx, `SELECT id FROM materials WHERE id=? AND class_id=? FOR SHARE`, m, u.ClassID).Scan(&found); err != nil {
			return Point{}, normalize(err)
		}
	}
	for _, a := range in.AssignmentIDs {
		var found int64
		if err = tx.QueryRowContext(ctx, `SELECT id FROM grade_assignments WHERE id=? AND class_id=? AND subject=? FOR SHARE`, a, u.ClassID, in.Subject).Scan(&found); err != nil {
			return Point{}, normalize(err)
		}
	}
	if id == 0 {
		r, e := tx.ExecContext(ctx, `INSERT INTO knowledge_points(class_id,subject,name,chapter,description,created_by) VALUES(?,?,?,?,?,?)`, u.ClassID, in.Subject, in.Name, in.Chapter, in.Description, u.ID)
		if e != nil {
			return Point{}, normalize(e)
		}
		id, err = r.LastInsertId()
		if err != nil {
			return Point{}, err
		}
	} else {
		if _, err = tx.ExecContext(ctx, `UPDATE knowledge_points SET subject=?,name=?,chapter=?,description=?,version=version+1,updated_at=UTC_TIMESTAMP(6) WHERE id=? AND class_id=?`, in.Subject, in.Name, in.Chapter, in.Description, id, u.ClassID); err != nil {
			return Point{}, normalize(err)
		}
	}
	for _, table := range []string{"knowledge_point_materials", "knowledge_point_assignments"} {
		if _, err = tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE point_id=?`, id); err != nil {
			return Point{}, err
		}
	}
	for _, m := range in.MaterialIDs {
		if _, err = tx.ExecContext(ctx, `INSERT INTO knowledge_point_materials(point_id,material_id) VALUES(?,?)`, id, m); err != nil {
			return Point{}, err
		}
	}
	for _, a := range in.AssignmentIDs {
		if _, err = tx.ExecContext(ctx, `INSERT INTO knowledge_point_assignments(point_id,assignment_id) VALUES(?,?)`, id, a); err != nil {
			return Point{}, err
		}
	}
	p, err := point(ctx, tx, u, id)
	if err != nil {
		return Point{}, err
	}
	if err = tx.Commit(); err != nil {
		return Point{}, err
	}
	return p, nil
}
func (s *Store) DeletePoint(ctx context.Context, u auth.User, id int64, expected int) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err = tx.QueryRowContext(ctx, `SELECT version FROM knowledge_points WHERE id=? AND class_id=? FOR UPDATE`, id, u.ClassID).Scan(&version); err != nil {
		return normalize(err)
	}
	if version != expected {
		return ErrConflict
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM knowledge_points WHERE id=? AND class_id=?`, id, u.ClassID); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) Favorite(ctx context.Context, u auth.User, id int64, add bool) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var found int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM knowledge_points WHERE id=? AND class_id=? FOR SHARE`, id, u.ClassID).Scan(&found); err != nil {
		return normalize(err)
	}
	if add {
		_, err = tx.ExecContext(ctx, `INSERT INTO knowledge_favorites(point_id,student_id) VALUES(?,?) ON DUPLICATE KEY UPDATE point_id=VALUES(point_id)`, id, u.ID)
	} else {
		_, err = tx.ExecContext(ctx, `DELETE FROM knowledge_favorites WHERE point_id=? AND student_id=?`, id, u.ID)
	}
	if err != nil {
		return fmt.Errorf("favorite: %w", err)
	}
	return tx.Commit()
}
