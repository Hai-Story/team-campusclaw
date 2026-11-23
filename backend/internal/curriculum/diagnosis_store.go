package curriculum

import (
	"campusclaw/backend/internal/auth"
	"context"
	"database/sql"
	"encoding/json"
)

const finalLatest = `NOT EXISTS(SELECT 1 FROM submissions n WHERE n.assignment_id=s.assignment_id AND n.student_id=s.student_id AND n.id>s.id)`
const diagnosisJoins = ` FROM submissions s JOIN users u ON u.id=s.student_id AND u.class_id=s.class_id AND u.role='student' JOIN grade_assignments a ON a.id=s.assignment_id AND a.class_id=s.class_id JOIN grade_reviews r ON r.submission_id=s.id AND r.class_id=s.class_id `
const diagnosisColumns = `s.id,s.assignment_id,s.student_id,u.username,a.title,r.score,r.max_score,r.correct,r.version,s.created_at,r.updated_at,COALESCE(r.error_tags,JSON_ARRAY())`

func (s *Store) Diagnosis(ctx context.Context, u auth.User, f DiagnosisFilter) (DiagnosisReport, error) {
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if e != nil {
		return DiagnosisReport{}, e
	}
	defer tx.Rollback()
	var role string
	var class int64
	if e = tx.QueryRowContext(ctx, `SELECT role,class_id FROM users WHERE id=?`, u.ID).Scan(&role, &class); e != nil {
		return DiagnosisReport{}, normalize(e)
	}
	if role != u.Role || class != u.ClassID {
		return DiagnosisReport{}, ErrNotFound
	}
	if role == "student" {
		if f.StudentID != 0 {
			return DiagnosisReport{}, ErrInvalid
		}
		f.StudentID = u.ID
	} else if role != "teacher" {
		return DiagnosisReport{}, ErrNotFound
	}
	students := []DiagnosisStudent{}
	rows, e := tx.QueryContext(ctx, `SELECT id,username FROM users WHERE class_id=? AND role='student' AND (?=0 OR id=?) ORDER BY id`, u.ClassID, func() int64 {
		if role == "student" {
			return u.ID
		}
		return 0
	}(), u.ID)
	if e != nil {
		return DiagnosisReport{}, e
	}
	selected := f.StudentID == 0
	for rows.Next() {
		var v DiagnosisStudent
		if e = rows.Scan(&v.ID, &v.Name); e != nil {
			rows.Close()
			return DiagnosisReport{}, e
		}
		students = append(students, v)
		if v.ID == f.StudentID {
			selected = true
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return DiagnosisReport{}, e
	}
	if !selected {
		return DiagnosisReport{}, ErrNotFound
	}
	points := []PointStats{}
	byAssignment := map[int64][]int64{}
	rows, e = tx.QueryContext(ctx, `SELECT id,name FROM knowledge_points WHERE class_id=? AND subject=? ORDER BY id`, u.ClassID, f.Subject)
	if e != nil {
		return DiagnosisReport{}, e
	}
	for rows.Next() {
		var p PointStats
		if e = rows.Scan(&p.ID, &p.Name); e != nil {
			rows.Close()
			return DiagnosisReport{}, e
		}
		points = append(points, p)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return DiagnosisReport{}, e
	}
	rows, e = tx.QueryContext(ctx, `SELECT pa.assignment_id,p.id FROM knowledge_point_assignments pa JOIN knowledge_points p ON p.id=pa.point_id WHERE p.class_id=? AND p.subject=? ORDER BY pa.assignment_id,p.id`, u.ClassID, f.Subject)
	if e != nil {
		return DiagnosisReport{}, e
	}
	for rows.Next() {
		var aid, pid int64
		if e = rows.Scan(&aid, &pid); e != nil {
			rows.Close()
			return DiagnosisReport{}, e
		}
		byAssignment[aid] = append(byAssignment[aid], pid)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return DiagnosisReport{}, e
	}
	samples := []DiagnosisSample{}
	rows, e = tx.QueryContext(ctx, `SELECT `+diagnosisColumns+diagnosisJoins+` WHERE s.class_id=? AND a.subject=? AND s.created_at>=? AND s.created_at<? AND (?=0 OR s.student_id=?) AND `+finalLatest+` ORDER BY s.id DESC LIMIT 10001`, u.ClassID, f.Subject, f.Start, f.End, f.StudentID, f.StudentID)
	if e != nil {
		return DiagnosisReport{}, e
	}
	for rows.Next() {
		var v DiagnosisSample
		var raw []byte
		if e = rows.Scan(&v.SubmissionID, &v.AssignmentID, &v.StudentID, &v.StudentName, &v.Title, &v.Score, &v.MaxScore, &v.Correct, &v.ReviewVersion, &v.SubmittedAt, &v.ReviewedAt, &raw); e != nil {
			rows.Close()
			return DiagnosisReport{}, e
		}
		if e = json.Unmarshal(raw, &v.ErrorTags); e != nil {
			rows.Close()
			return DiagnosisReport{}, e
		}
		v.PointIDs = append([]int64{}, byAssignment[v.AssignmentID]...)
		samples = append(samples, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return DiagnosisReport{}, e
	}
	if len(samples) > 10000 {
		return DiagnosisReport{}, ErrDiagnosisSize
	}
	result := summarizeDiagnosis(f, samples, points, students)
	if e = tx.Commit(); e != nil {
		return DiagnosisReport{}, e
	}
	return result, nil
}
func (s *Store) DiagnosisSample(ctx context.Context, u auth.User, id int64) (DiagnosisSample, error) {
	var v DiagnosisSample
	if u.Role != "teacher" && u.Role != "student" {
		return v, ErrNotFound
	}
	var raw []byte
	student := int64(0)
	if u.Role == "student" {
		student = u.ID
	}
	e := s.DB.QueryRowContext(ctx, `SELECT `+diagnosisColumns+`,a.question,s.answer,r.feedback`+diagnosisJoins+` WHERE s.id=? AND s.class_id=? AND (?=0 OR s.student_id=?) AND `+finalLatest, id, u.ClassID, student, student).Scan(&v.SubmissionID, &v.AssignmentID, &v.StudentID, &v.StudentName, &v.Title, &v.Score, &v.MaxScore, &v.Correct, &v.ReviewVersion, &v.SubmittedAt, &v.ReviewedAt, &raw, &v.Question, &v.Answer, &v.Feedback)
	if e != nil {
		return v, normalize(e)
	}
	e = json.Unmarshal(raw, &v.ErrorTags)
	if e != nil {
		return v, e
	}
	v.PointIDs = []int64{}
	rows, e := s.DB.QueryContext(ctx, `SELECT p.id FROM knowledge_point_assignments pa JOIN knowledge_points p ON p.id=pa.point_id JOIN grade_assignments a ON a.id=pa.assignment_id AND a.class_id=p.class_id AND a.subject=p.subject WHERE pa.assignment_id=? AND p.class_id=? ORDER BY p.id`, v.AssignmentID, u.ClassID)
	if e != nil {
		return v, e
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if e = rows.Scan(&id); e != nil {
			return v, e
		}
		v.PointIDs = append(v.PointIDs, id)
	}
	return v, rows.Err()
}
