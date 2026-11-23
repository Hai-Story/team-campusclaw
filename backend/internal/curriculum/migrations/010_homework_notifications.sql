CREATE TABLE IF NOT EXISTS student_homework_notifications (
 id BIGINT AUTO_INCREMENT PRIMARY KEY,
 class_id BIGINT NOT NULL, created_by BIGINT NOT NULL, student_id BIGINT NOT NULL,
 assignment_id BIGINT NULL, paper_id BIGINT NULL,
 read_at DATETIME(6) NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 UNIQUE KEY uq_notice_assignment(student_id,assignment_id),
 UNIQUE KEY uq_notice_paper(student_id,paper_id),
 INDEX idx_notice_student(class_id,student_id,id),
 INDEX idx_notice_unread(class_id,student_id,read_at),
 FOREIGN KEY(class_id) REFERENCES classes(id),
 FOREIGN KEY(created_by) REFERENCES users(id) ON DELETE CASCADE,
 FOREIGN KEY(student_id) REFERENCES users(id) ON DELETE CASCADE,
 FOREIGN KEY(assignment_id) REFERENCES grade_assignments(id) ON DELETE CASCADE,
 FOREIGN KEY(paper_id) REFERENCES published_papers(id) ON DELETE CASCADE,
 CHECK((assignment_id IS NULL) <> (paper_id IS NULL))
);
