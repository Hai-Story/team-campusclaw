package homeworknotice

import (
	"context"
	"database/sql"
)

// Enqueue belongs to the publication transaction; paper children never call it.
func Enqueue(ctx context.Context, tx *sql.Tx, classID, teacher, assignment, paper int64) error {
	var a, p any
	if assignment > 0 {
		a = assignment
	}
	if paper > 0 {
		p = paper
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO student_homework_notifications(class_id,created_by,student_id,assignment_id,paper_id)
 SELECT ?,?,id,?,? FROM users WHERE class_id=? AND role='student'`, classID, teacher, a, p, classID)
	return err
}
