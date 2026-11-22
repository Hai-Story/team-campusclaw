package curriculum

import (
	"campusclaw/backend/internal/auth"
	"context"
	"database/sql"
)

func graphScope(ctx context.Context, tx *sql.Tx, u auth.User, subject string) error {
	if _, e := tx.ExecContext(ctx, `INSERT INTO knowledge_graph_scopes(class_id,subject,created_by) VALUES(?,?,?) ON DUPLICATE KEY UPDATE id=id`, u.ClassID, subject, u.ID); e != nil {
		return e
	}
	var id int64
	return tx.QueryRowContext(ctx, `SELECT id FROM knowledge_graph_scopes WHERE class_id=? AND subject=? FOR UPDATE`, u.ClassID, subject).Scan(&id)
}
func graphEdges(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, class int64, subject string) ([]GraphEdge, error) {
	rows, e := q.QueryContext(ctx, `SELECT id,from_id,to_id,kind,version FROM knowledge_graph_edges WHERE class_id=? AND subject=? ORDER BY id`, class, subject)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	v := []GraphEdge{}
	for rows.Next() {
		var edge GraphEdge
		if e = rows.Scan(&edge.ID, &edge.FromID, &edge.ToID, &edge.Kind, &edge.Version); e != nil {
			return nil, e
		}
		v = append(v, edge)
	}
	return v, rows.Err()
}
func insertGraphEdge(ctx context.Context, tx *sql.Tx, u auth.User, subject string, in EdgeInput, edges *[]GraphEdge) (GraphEdge, error) {
	for _, edge := range *edges {
		if edge.FromID == in.FromID && edge.ToID == in.ToID && edge.Kind == in.Kind {
			return edge, nil
		}
	}
	if len(*edges) >= 2000 || createsCycle(*edges, in.FromID, in.ToID, in.Kind) {
		return GraphEdge{}, ErrConflict
	}
	r, e := tx.ExecContext(ctx, `INSERT INTO knowledge_graph_edges(class_id,subject,from_id,to_id,kind,created_by) VALUES(?,?,?,?,?,?)`, u.ClassID, subject, in.FromID, in.ToID, in.Kind, u.ID)
	if e != nil {
		return GraphEdge{}, normalize(e)
	}
	id, e := r.LastInsertId()
	v := GraphEdge{ID: id, FromID: in.FromID, ToID: in.ToID, Kind: in.Kind, Version: 1}
	*edges = append(*edges, v)
	return v, e
}
func (s *Store) AddEdge(ctx context.Context, u auth.User, in EdgeInput) (GraphEdge, error) {
	var subject string
	if e := s.DB.QueryRowContext(ctx, `SELECT subject FROM knowledge_points WHERE id=? AND class_id=?`, in.FromID, u.ClassID).Scan(&subject); e != nil {
		return GraphEdge{}, normalize(e)
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return GraphEdge{}, e
	}
	defer tx.Rollback()
	if e = graphScope(ctx, tx, u, subject); e != nil {
		return GraphEdge{}, e
	}
	for _, id := range []int64{in.FromID, in.ToID} {
		var found int64
		if e = tx.QueryRowContext(ctx, `SELECT id FROM knowledge_points WHERE id=? AND class_id=? AND subject=? FOR SHARE`, id, u.ClassID, subject).Scan(&found); e != nil {
			return GraphEdge{}, normalize(e)
		}
	}
	edges, e := graphEdges(ctx, tx, u.ClassID, subject)
	if e != nil {
		return GraphEdge{}, e
	}
	v, e := insertGraphEdge(ctx, tx, u, subject, in, &edges)
	if e != nil {
		return v, e
	}
	return v, tx.Commit()
}
func (s *Store) DeleteEdge(ctx context.Context, u auth.User, id int64, expected int) error {
	var subject string
	if e := s.DB.QueryRowContext(ctx, `SELECT subject FROM knowledge_graph_edges WHERE id=? AND class_id=?`, id, u.ClassID).Scan(&subject); e != nil {
		return normalize(e)
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = graphScope(ctx, tx, u, subject); e != nil {
		return e
	}
	var version int
	if e = tx.QueryRowContext(ctx, `SELECT version FROM knowledge_graph_edges WHERE id=? AND class_id=? FOR UPDATE`, id, u.ClassID).Scan(&version); e != nil {
		return normalize(e)
	}
	if version != expected {
		return ErrConflict
	}
	if _, e = tx.ExecContext(ctx, `DELETE FROM knowledge_graph_edges WHERE id=?`, id); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) mastery(ctx context.Context, u auth.User, subject string) (map[int64]Mastery, error) {
	query := `SELECT p.id,COUNT(*),SUM(r.correct=0) FROM knowledge_points p JOIN knowledge_point_assignments pa ON pa.point_id=p.id JOIN submissions s ON s.assignment_id=pa.assignment_id AND s.class_id=p.class_id JOIN grade_reviews r ON r.submission_id=s.id AND r.class_id=s.class_id WHERE p.class_id=? AND p.subject=? AND NOT EXISTS(SELECT 1 FROM submissions newer WHERE newer.assignment_id=s.assignment_id AND newer.student_id=s.student_id AND newer.class_id=s.class_id AND newer.id>s.id)`
	args := []any{u.ClassID, subject}
	if u.Role == "student" {
		query += ` AND s.student_id=?`
		args = append(args, u.ID)
	}
	query += ` GROUP BY p.id`
	rows, e := s.DB.QueryContext(ctx, query, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := map[int64]Mastery{}
	for rows.Next() {
		var id int64
		var m Mastery
		if e = rows.Scan(&id, &m.Reviewed, &m.Incorrect); e != nil {
			return nil, e
		}
		m.Weak = m.Reviewed > 0 && m.Incorrect*2 >= m.Reviewed
		out[id] = m
	}
	return out, rows.Err()
}
func (s *Store) Graph(ctx context.Context, u auth.User, subject string) (Graph, error) {
	out := Graph{Subject: subject, Nodes: []GraphNode{}, Edges: []GraphEdge{}}
	rows, e := s.DB.QueryContext(ctx, `SELECT id,name,chapter FROM knowledge_points WHERE class_id=? AND subject=? ORDER BY id LIMIT 501`, u.ClassID, subject)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var n GraphNode
		if e = rows.Scan(&n.ID, &n.Name, &n.Chapter); e != nil {
			rows.Close()
			return out, e
		}
		out.Nodes = append(out.Nodes, n)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	if len(out.Nodes) > 500 {
		return out, ErrConflict
	}
	out.Edges, e = graphEdges(ctx, s.DB, u.ClassID, subject)
	if e != nil {
		return out, e
	}
	m, e := s.mastery(ctx, u, subject)
	if e != nil {
		return out, e
	}
	for i := range out.Nodes {
		out.Nodes[i].Mastery = m[out.Nodes[i].ID]
	}
	return out, nil
}

type GraphResource struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}
type GraphMistake struct {
	SubmissionID int64  `json:"submission_id"`
	AssignmentID int64  `json:"assignment_id"`
	Title        string `json:"title"`
	Answer       string `json:"answer"`
}
type NodeResources struct {
	Point           Point           `json:"point"`
	Materials       []GraphResource `json:"materials"`
	Assignments     []GraphResource `json:"assignments"`
	Recommended     []GraphNode     `json:"recommended_points"`
	ReviewMaterials []GraphResource `json:"review_materials"`
	Mistakes        []GraphMistake  `json:"mistakes"`
	Mastery
}

func resources(ctx context.Context, db *sql.DB, query string, args ...any) ([]GraphResource, error) {
	rows, e := db.QueryContext(ctx, query, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []GraphResource{}
	for rows.Next() {
		var v GraphResource
		if e = rows.Scan(&v.ID, &v.Title); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) NodeResources(ctx context.Context, u auth.User, id int64) (NodeResources, error) {
	out := NodeResources{Materials: []GraphResource{}, Assignments: []GraphResource{}, Recommended: []GraphNode{}, ReviewMaterials: []GraphResource{}, Mistakes: []GraphMistake{}}
	p, e := s.Point(ctx, u, id)
	if e != nil {
		return out, e
	}
	out.Point = p
	m, e := s.mastery(ctx, u, p.Subject)
	if e != nil {
		return out, e
	}
	out.Mastery = m[id]
	out.Materials, e = resources(ctx, s.DB, `SELECT m.id,m.title FROM knowledge_point_materials pm JOIN materials m ON m.id=pm.material_id WHERE pm.point_id=? AND m.class_id=? ORDER BY m.id`, id, u.ClassID)
	if e != nil {
		return out, e
	}
	out.Assignments, e = resources(ctx, s.DB, `SELECT a.id,a.title FROM knowledge_point_assignments pa JOIN grade_assignments a ON a.id=pa.assignment_id WHERE pa.point_id=? AND a.class_id=? AND a.subject=? ORDER BY a.id`, id, u.ClassID, p.Subject)
	if e != nil {
		return out, e
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT p.id,p.name,p.chapter FROM knowledge_graph_edges e JOIN knowledge_points p ON p.id=e.from_id WHERE e.to_id=? AND e.class_id=? AND e.subject=? AND e.kind='prerequisite' AND p.class_id=e.class_id AND p.subject=e.subject ORDER BY p.id`, id, u.ClassID, p.Subject)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var n GraphNode
		if e = rows.Scan(&n.ID, &n.Name, &n.Chapter); e != nil {
			rows.Close()
			return out, e
		}
		n.Mastery = m[n.ID]
		out.Recommended = append(out.Recommended, n)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	out.ReviewMaterials, e = resources(ctx, s.DB, `SELECT DISTINCT m.id,m.title FROM knowledge_point_materials pm JOIN materials m ON m.id=pm.material_id WHERE m.class_id=? AND (pm.point_id=? OR pm.point_id IN (SELECT from_id FROM knowledge_graph_edges WHERE to_id=? AND class_id=? AND subject=? AND kind='prerequisite')) ORDER BY m.id`, u.ClassID, id, id, u.ClassID, p.Subject)
	if e != nil {
		return out, e
	}
	if u.Role == "student" {
		rows, e = s.DB.QueryContext(ctx, `SELECT s.id,a.id,a.title,s.answer FROM knowledge_point_assignments pa JOIN grade_assignments a ON a.id=pa.assignment_id JOIN submissions s ON s.assignment_id=a.id LEFT JOIN grade_results g ON g.submission_id=s.id LEFT JOIN grade_reviews r ON r.submission_id=s.id WHERE pa.point_id=? AND a.class_id=? AND a.subject=? AND s.class_id=? AND s.student_id=? AND COALESCE(r.correct,g.ai_correct)=false AND NOT EXISTS(SELECT 1 FROM submissions newer WHERE newer.assignment_id=s.assignment_id AND newer.student_id=s.student_id AND newer.id>s.id) ORDER BY s.id DESC`, id, u.ClassID, p.Subject, u.ClassID, u.ID)
		if e != nil {
			return out, e
		}
		defer rows.Close()
		for rows.Next() {
			var v GraphMistake
			if e = rows.Scan(&v.SubmissionID, &v.AssignmentID, &v.Title, &v.Answer); e != nil {
				return out, e
			}
			out.Mistakes = append(out.Mistakes, v)
		}
		if e = rows.Err(); e != nil {
			return out, e
		}
	}
	return out, nil
}
