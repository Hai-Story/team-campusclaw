package curriculum

import (
	"campusclaw/backend/internal/auth"
	"context"
	"database/sql"
	"time"
)

type HomeworkNotification struct {
	ID         int64      `json:"id"`
	Kind       string     `json:"kind"`
	ResourceID int64      `json:"resource_id"`
	Subject    string     `json:"subject"`
	Title      string     `json:"title"`
	CreatedAt  time.Time  `json:"created_at"`
	ReadAt     *time.Time `json:"read_at"`
}
type HomeworkNotifications struct {
	Items       []HomeworkNotification `json:"items"`
	Page        int                    `json:"page"`
	PageSize    int                    `json:"page_size"`
	UnreadCount int                    `json:"unread_count"`
}

func (s *Store) HomeworkNotifications(ctx context.Context, u auth.User, page, size int) (HomeworkNotifications, error) {
	out := HomeworkNotifications{Items: []HomeworkNotification{}, Page: page, PageSize: size}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if e = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM student_homework_notifications WHERE class_id=? AND student_id=? AND read_at IS NULL`, u.ClassID, u.ID).Scan(&out.UnreadCount); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT n.id,IF(n.paper_id IS NULL,'assignment','paper'),COALESCE(n.assignment_id,n.paper_id),COALESCE(a.subject,p.subject),COALESCE(a.title,p.title),n.created_at,n.read_at
 FROM student_homework_notifications n LEFT JOIN grade_assignments a ON a.id=n.assignment_id LEFT JOIN published_papers p ON p.id=n.paper_id
 WHERE n.class_id=? AND n.student_id=? ORDER BY n.id DESC LIMIT ? OFFSET ?`, u.ClassID, u.ID, size, int64(page-1)*int64(size))
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var v HomeworkNotification
		if e = rows.Scan(&v.ID, &v.Kind, &v.ResourceID, &v.Subject, &v.Title, &v.CreatedAt, &v.ReadAt); e != nil {
			rows.Close()
			return out, e
		}
		out.Items = append(out.Items, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}
func (s *Store) ReadHomeworkNotification(ctx context.Context, u auth.User, id int64) error {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var found int64
	if e = tx.QueryRowContext(ctx, `SELECT id FROM student_homework_notifications WHERE id=? AND class_id=? AND student_id=? FOR UPDATE`, id, u.ClassID, u.ID).Scan(&found); e != nil {
		return normalize(e)
	}
	if _, e = tx.ExecContext(ctx, `UPDATE student_homework_notifications SET updated_at=IF(read_at IS NULL,CURRENT_TIMESTAMP(6),updated_at),read_at=COALESCE(read_at,CURRENT_TIMESTAMP(6)) WHERE id=?`, found); e != nil {
		return e
	}
	return tx.Commit()
}
