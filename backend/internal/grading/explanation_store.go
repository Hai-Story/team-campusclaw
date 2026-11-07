package grading

import (
	"context"
	"database/sql"
)

func (s *Store) Mistake(ctx context.Context, classID, studentID, submissionID int64) (Work, error) {
	var w Work
	err := s.DB.QueryRowContext(ctx, `SELECT s.id,s.class_id,s.student_id,a.subject,a.question,a.expected_answer,s.answer FROM submissions s JOIN users u ON u.id=s.student_id AND u.class_id=s.class_id AND u.role='student' JOIN grade_assignments a ON a.id=s.assignment_id AND a.class_id=s.class_id JOIN grade_tasks t ON t.submission_id=s.id LEFT JOIN grade_results r ON r.idempotency_key=t.idempotency_key LEFT JOIN grade_reviews f ON f.submission_id=s.id AND f.class_id=s.class_id WHERE s.id=? AND s.class_id=? AND s.student_id=? AND COALESCE(f.correct,r.ai_correct)=FALSE AND `+latestSubmission, submissionID, classID, studentID).Scan(&w.SubmissionID, &w.ClassID, &w.ActorID, &w.Subject, &w.Question, &w.Expected, &w.Answer)
	if err == sql.ErrNoRows {
		err = ErrNotFound
	}
	return w, err
}
