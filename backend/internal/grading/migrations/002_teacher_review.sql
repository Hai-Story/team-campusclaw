ALTER TABLE grade_results MODIFY COLUMN score INT NULL;
CREATE TABLE IF NOT EXISTS grade_reviews (
 id BIGINT AUTO_INCREMENT PRIMARY KEY,
 submission_id BIGINT NOT NULL UNIQUE,
 class_id BIGINT NOT NULL,
 created_by BIGINT NOT NULL,
 score INT NOT NULL,
 max_score INT NOT NULL,
 correct BOOLEAN NOT NULL,
 feedback TEXT NOT NULL,
 version INT NOT NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 FOREIGN KEY(submission_id) REFERENCES submissions(id) ON DELETE CASCADE,
 FOREIGN KEY(class_id) REFERENCES classes(id),
 FOREIGN KEY(created_by) REFERENCES users(id),
 CHECK(score>=0 AND score<=max_score),
 CHECK(version>=1)
);
CREATE TABLE IF NOT EXISTS grade_review_history (
 id BIGINT AUTO_INCREMENT PRIMARY KEY,
 submission_id BIGINT NOT NULL,
 class_id BIGINT NOT NULL,
 created_by BIGINT NOT NULL,
 score INT NOT NULL,
 max_score INT NOT NULL,
 correct BOOLEAN NOT NULL,
 feedback TEXT NOT NULL,
 version INT NOT NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 UNIQUE KEY uq_review_version(submission_id,version),
 FOREIGN KEY(submission_id) REFERENCES submissions(id) ON DELETE CASCADE,
 FOREIGN KEY(class_id) REFERENCES classes(id),
 FOREIGN KEY(created_by) REFERENCES users(id),
 CHECK(score>=0 AND score<=max_score),
 CHECK(version>=1)
);
