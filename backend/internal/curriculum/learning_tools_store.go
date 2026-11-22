package curriculum

import (
	"campusclaw/backend/internal/auth"
	"context"
)

func (s *Store) CheckLearningActor(ctx context.Context, u auth.User) error {
	var id int64
	e := s.DB.QueryRowContext(ctx, `SELECT id FROM users WHERE id=? AND class_id=? AND role=? AND role IN ('teacher','student')`, u.ID, u.ClassID, u.Role).Scan(&id)
	return normalize(e)
}
