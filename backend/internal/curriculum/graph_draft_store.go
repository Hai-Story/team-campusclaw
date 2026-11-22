package curriculum

import (
	"campusclaw/backend/internal/auth"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

const graphDraftColumns = `id,subject,input_json,existing_nodes_json,nodes_json,edges_json,citations_json,point_ids_json,status,version,created_at,updated_at`

func scanGraphDraft(row scanner) (GraphDraft, error) {
	var v GraphDraft
	var input, existing, nodes, edges, cites, ids []byte
	e := row.Scan(&v.ID, &v.Subject, &input, &existing, &nodes, &edges, &cites, &ids, &v.Status, &v.Version, &v.CreatedAt, &v.UpdatedAt)
	if e != nil {
		return v, normalize(e)
	}
	for _, pair := range []struct {
		b []byte
		v any
	}{{input, &v.Input}, {existing, &v.Existing}, {nodes, &v.Nodes}, {cites, &v.Citations}, {ids, &v.PointIDs}} {
		if e = json.Unmarshal(pair.b, pair.v); e != nil {
			return v, e
		}
	}
	var payload struct {
		Edges   []CandidateEdge `json:"edges"`
		Indices []int           `json:"indices"`
	}
	if e = json.Unmarshal(edges, &payload); e != nil {
		return v, e
	}
	v.Edges = payload.Edges
	v.CitationIndices = payload.Indices
	return v, nil
}

// Citation indices are persisted with the node/edge payload, separate from immutable source snapshots.
func graphDraft(ctx context.Context, q queryer, u auth.User, id int64, lock bool) (GraphDraft, error) {
	query := `SELECT ` + graphDraftColumns + ` FROM knowledge_graph_drafts WHERE id=? AND class_id=? AND created_by=?`
	if lock {
		query += ` FOR UPDATE`
	}
	return scanGraphDraft(q.QueryRowContext(ctx, query, id, u.ClassID, u.ID))
}
func (s *Store) GraphDraft(ctx context.Context, u auth.User, id int64) (GraphDraft, error) {
	v, e := graphDraft(ctx, s.DB, u, id, false)
	if e != nil {
		return v, e
	}
	v.Citations, e = s.currentEvidence(ctx, u, v.Citations)
	return v, e
}
func (s *Store) GraphDraftByKey(ctx context.Context, u auth.User, in SourceInput) (GraphDraft, error) {
	var id int64
	var hash string
	e := s.DB.QueryRowContext(ctx, `SELECT id,input_hash FROM knowledge_graph_drafts WHERE class_id=? AND created_by=? AND client_key=?`, u.ClassID, u.ID, in.ClientKey).Scan(&id, &hash)
	if e != nil {
		return GraphDraft{}, normalize(e)
	}
	if hash != in.Hash() {
		return GraphDraft{}, ErrConflict
	}
	return s.GraphDraft(ctx, u, id)
}
func (s *Store) GraphDrafts(ctx context.Context, u auth.User, f PointFilter) ([]GraphDraft, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT `+graphDraftColumns+` FROM knowledge_graph_drafts WHERE class_id=? AND created_by=? AND subject=? AND CAST(nodes_json AS CHAR) LIKE ? ORDER BY id DESC LIMIT ? OFFSET ?`, u.ClassID, u.ID, f.Subject, "%"+f.Q+"%", f.Size, int64(f.Page-1)*int64(f.Size))
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []GraphDraft{}
	for rows.Next() {
		v, e := scanGraphDraft(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func graphCatalog(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, class int64, subject string) (map[string]int64, error) {
	rows, e := q.QueryContext(ctx, `SELECT name,id FROM knowledge_points WHERE class_id=? AND subject=? ORDER BY id`, class, subject)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var name string
		var id int64
		if e = rows.Scan(&name, &id); e != nil {
			return nil, e
		}
		out[name] = id
	}
	return out, rows.Err()
}
func validTeacher(ctx context.Context, tx *sql.Tx, u auth.User) error {
	var role string
	var class int64
	if e := tx.QueryRowContext(ctx, `SELECT role,class_id FROM users WHERE id=? FOR SHARE`, u.ID).Scan(&role, &class); e != nil {
		return e
	}
	if role != "teacher" || class != u.ClassID {
		return ErrNotFound
	}
	return nil
}
func checkEvidence(ctx context.Context, tx *sql.Tx, u auth.User, refs []Evidence) error {
	for _, c := range refs {
		live, e := evidence(ctx, tx, u, c.ChunkID, c.MaterialID, true)
		if e != nil {
			if errors.Is(e, ErrNotFound) {
				return ErrEvidence
			}
			return e
		}
		if live.Excerpt != c.Excerpt || live.CharStart != c.CharStart || live.CharEnd != c.CharEnd {
			return ErrEvidence
		}
	}
	return nil
}
func (s *Store) SaveGraphDraft(ctx context.Context, u auth.User, in SourceInput, out GraphOutput, refs []Evidence, catalog map[string]int64) (GraphDraft, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return GraphDraft{}, e
	}
	defer tx.Rollback()
	if e = validTeacher(ctx, tx, u); e != nil {
		return GraphDraft{}, e
	}
	if e = checkEvidence(ctx, tx, u, refs); e != nil {
		return GraphDraft{}, e
	}
	raw, _ := json.Marshal(in)
	existing, _ := json.Marshal(catalog)
	nodes, _ := json.Marshal(out.Nodes)
	payload, _ := json.Marshal(struct {
		Edges   []CandidateEdge `json:"edges"`
		Indices []int           `json:"indices"`
	}{out.Edges, out.CitationIndices})
	cites, _ := json.Marshal(refs)
	r, e := tx.ExecContext(ctx, `INSERT INTO knowledge_graph_drafts(class_id,created_by,subject,input_json,existing_nodes_json,nodes_json,edges_json,citations_json,point_ids_json,client_key,input_hash) VALUES(?,?,?,?,?,?,?,?,JSON_ARRAY(),?,?)`, u.ClassID, u.ID, in.Subject, string(raw), string(existing), string(nodes), string(payload), string(cites), in.ClientKey, in.Hash())
	if e != nil {
		tx.Rollback()
		if errors.Is(normalize(e), ErrConflict) {
			return s.GraphDraftByKey(ctx, u, in)
		}
		return GraphDraft{}, e
	}
	id, e := r.LastInsertId()
	if e != nil {
		return GraphDraft{}, e
	}
	if e = tx.Commit(); e != nil {
		return GraphDraft{}, e
	}
	return s.GraphDraft(ctx, u, id)
}
func (s *Store) UpdateGraphDraft(ctx context.Context, u auth.User, id int64, in GraphUpdate) (GraphDraft, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return GraphDraft{}, e
	}
	defer tx.Rollback()
	v, e := graphDraft(ctx, tx, u, id, true)
	if e != nil {
		return v, e
	}
	if v.Status != "pending" || v.Version != in.ExpectedVersion {
		return v, ErrConflict
	}
	catalog, e := graphCatalog(ctx, tx, u.ClassID, v.Subject)
	if e != nil {
		return v, e
	}
	if e = in.GraphOutput.Validate(catalog, len(v.Citations)); e != nil {
		return v, e
	}
	nodes, _ := json.Marshal(in.Nodes)
	payload, _ := json.Marshal(struct {
		Edges   []CandidateEdge `json:"edges"`
		Indices []int           `json:"indices"`
	}{in.Edges, in.CitationIndices})
	if _, e = tx.ExecContext(ctx, `UPDATE knowledge_graph_drafts SET nodes_json=?,edges_json=?,version=version+1,updated_at=UTC_TIMESTAMP(6) WHERE id=?`, string(nodes), string(payload), id); e != nil {
		return v, e
	}
	v, e = graphDraft(ctx, tx, u, id, false)
	if e != nil {
		return v, e
	}
	if e = tx.Commit(); e != nil {
		return v, e
	}
	return v, nil
}
func (s *Store) ApproveGraphDraft(ctx context.Context, u auth.User, id int64, version int) (GraphDraft, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return GraphDraft{}, e
	}
	defer tx.Rollback()
	v, e := graphDraft(ctx, tx, u, id, true)
	if e != nil {
		return v, e
	}
	if v.Status == "approved" && (version == v.Version || version == v.Version-1) {
		return v, nil
	}
	if v.Status != "pending" || v.Version != version {
		return v, ErrConflict
	}
	if e = graphScope(ctx, tx, u, v.Subject); e != nil {
		return v, e
	}
	if e = validTeacher(ctx, tx, u); e != nil {
		return v, e
	}
	selected := []Evidence{}
	for _, i := range v.CitationIndices {
		if i < 1 || i > len(v.Citations) {
			return v, ErrConflict
		}
		selected = append(selected, v.Citations[i-1])
	}
	if e = checkEvidence(ctx, tx, u, selected); e != nil {
		return v, e
	}
	catalog, e := graphCatalog(ctx, tx, u.ClassID, v.Subject)
	if e != nil {
		return v, e
	}
	if e = v.GraphOutput.Validate(catalog, len(v.Citations)); e != nil {
		return v, ErrConflict
	}
	names := map[string]int64{}
	for name, id := range catalog {
		names[strings.ToLower(name)] = id
	}

	referenced := map[string]bool{}
	for _, n := range v.Nodes {
		referenced[strings.ToLower(n.Name)] = true
	}
	for _, edge := range v.Edges {
		referenced[strings.ToLower(edge.From)] = true
		referenced[strings.ToLower(edge.To)] = true
	}
	for name, pid := range v.Existing {
		if !referenced[strings.ToLower(name)] {
			continue
		}
		var currentName string
		if e = tx.QueryRowContext(ctx, `SELECT name FROM knowledge_points WHERE id=? AND class_id=? AND subject=? FOR SHARE`, pid, u.ClassID, v.Subject).Scan(&currentName); e != nil {
			if errors.Is(e, sql.ErrNoRows) {
				return v, ErrConflict
			}
			return v, e
		}
		if !strings.EqualFold(name, currentName) {
			return v, ErrConflict
		}
	}
	ids := []int64{}
	count := len(catalog)
	for _, n := range v.Nodes {
		key := strings.ToLower(n.Name)
		pid := names[key]
		if pid == 0 {
			if count >= 500 {
				return v, ErrConflict
			}
			r, e := tx.ExecContext(ctx, `INSERT INTO knowledge_points(class_id,subject,name,chapter,description,created_by) VALUES(?,?,?,?,?,?)`, u.ClassID, v.Subject, n.Name, n.Chapter, n.Description, u.ID)
			if e != nil {
				return v, normalize(e)
			}
			pid, e = r.LastInsertId()
			if e != nil {
				return v, e
			}
			names[key] = pid
			count++
		}
		for _, c := range selected {
			if _, e = tx.ExecContext(ctx, `INSERT IGNORE INTO knowledge_point_materials(point_id,material_id) VALUES(?,?)`, pid, c.MaterialID); e != nil {
				return v, e
			}
		}
		var links int
		if e = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge_point_materials WHERE point_id=?`, pid).Scan(&links); e != nil {
			return v, e
		}
		if links > 50 {
			return v, ErrConflict
		}
		ids = append(ids, pid)
	}
	edges, e := graphEdges(ctx, tx, u.ClassID, v.Subject)
	if e != nil {
		return v, e
	}
	for _, edge := range v.Edges {
		in := EdgeInput{FromID: names[strings.ToLower(edge.From)], ToID: names[strings.ToLower(edge.To)], Kind: edge.Kind}
		if e = in.Validate(); e != nil {
			return v, ErrConflict
		}
		if _, e = insertGraphEdge(ctx, tx, u, v.Subject, in, &edges); e != nil {
			return v, e
		}
	}
	b, _ := json.Marshal(ids)
	if _, e = tx.ExecContext(ctx, `UPDATE knowledge_graph_drafts SET status='approved',version=version+1,point_ids_json=?,updated_at=UTC_TIMESTAMP(6) WHERE id=?`, string(b), id); e != nil {
		return v, e
	}
	v, e = graphDraft(ctx, tx, u, id, false)
	if e != nil {
		return v, e
	}
	return v, tx.Commit()
}

func (s *Store) GraphCatalog(ctx context.Context, u auth.User, subject string) (map[string]int64,error) { return graphCatalog(ctx,s.DB,u.ClassID,subject) }
