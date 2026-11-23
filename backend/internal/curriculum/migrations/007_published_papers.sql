CREATE TABLE IF NOT EXISTS published_papers (
 id BIGINT AUTO_INCREMENT PRIMARY KEY,
 class_id BIGINT NOT NULL, created_by BIGINT NOT NULL,
 draft_id BIGINT NULL, draft_version INT NOT NULL,
 subject VARCHAR(16) NOT NULL, title VARCHAR(120) NOT NULL,
 total_score INT NOT NULL, created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 UNIQUE KEY uq_published_version(draft_id,draft_version),
 INDEX idx_published_class(class_id,subject,id),
 FOREIGN KEY(class_id) REFERENCES classes(id), FOREIGN KEY(created_by) REFERENCES users(id),
 FOREIGN KEY(draft_id) REFERENCES paper_drafts(id) ON DELETE SET NULL,
 CHECK(total_score BETWEEN 1 AND 1000)
);
CREATE TABLE IF NOT EXISTS published_paper_questions (
 paper_id BIGINT NOT NULL, position INT NOT NULL, assignment_id BIGINT NOT NULL,
 item_json JSON NOT NULL,
 PRIMARY KEY(paper_id,position), UNIQUE KEY uq_paper_assignment(assignment_id),
 FOREIGN KEY(paper_id) REFERENCES published_papers(id) ON DELETE CASCADE,
 FOREIGN KEY(assignment_id) REFERENCES grade_assignments(id),
 CHECK(position BETWEEN 1 AND 50)
);
CREATE TABLE IF NOT EXISTS paper_attempts (
 id BIGINT AUTO_INCREMENT PRIMARY KEY, paper_id BIGINT NOT NULL,
 class_id BIGINT NOT NULL, student_id BIGINT NOT NULL,
 client_key VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 input_hash CHAR(64) CHARACTER SET ascii NOT NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 UNIQUE KEY uq_paper_attempt(paper_id,student_id,client_key),
 INDEX idx_attempt_latest(paper_id,student_id,id),
 FOREIGN KEY(paper_id) REFERENCES published_papers(id) ON DELETE CASCADE,
 FOREIGN KEY(class_id) REFERENCES classes(id), FOREIGN KEY(student_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS paper_attempt_submissions (
 attempt_id BIGINT NOT NULL, assignment_id BIGINT NOT NULL, submission_id BIGINT NOT NULL,
 PRIMARY KEY(attempt_id,assignment_id), UNIQUE KEY uq_paper_submission(submission_id),
 FOREIGN KEY(attempt_id) REFERENCES paper_attempts(id) ON DELETE CASCADE,
 FOREIGN KEY(assignment_id) REFERENCES grade_assignments(id),
 FOREIGN KEY(submission_id) REFERENCES submissions(id) ON DELETE CASCADE
);
