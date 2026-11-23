CREATE TABLE IF NOT EXISTS review_settings (
 id BIGINT AUTO_INCREMENT PRIMARY KEY,
 student_id BIGINT NOT NULL, class_id BIGINT NOT NULL, created_by BIGINT NOT NULL,
 intervals_json JSON NOT NULL, version INT NOT NULL DEFAULT 1,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 UNIQUE KEY uq_review_settings(student_id,class_id),
 FOREIGN KEY(student_id) REFERENCES users(id) ON DELETE CASCADE,
 FOREIGN KEY(class_id) REFERENCES classes(id), FOREIGN KEY(created_by) REFERENCES users(id)
);
CREATE TABLE IF NOT EXISTS review_cards (
 id BIGINT AUTO_INCREMENT PRIMARY KEY,
 student_id BIGINT NOT NULL, class_id BIGINT NOT NULL, created_by BIGINT NOT NULL,
 source_kind VARCHAR(16) NOT NULL, source_id BIGINT NOT NULL,
 subject VARCHAR(16) NOT NULL, title VARCHAR(120) NOT NULL, body MEDIUMTEXT NOT NULL,
 level INT NOT NULL DEFAULT 0, due_at DATETIME(6) NOT NULL, version INT NOT NULL DEFAULT 1,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 UNIQUE KEY uq_review_source(student_id,source_kind,source_id),
 INDEX idx_review_due(student_id,class_id,subject,due_at,id),
 FOREIGN KEY(student_id) REFERENCES users(id) ON DELETE CASCADE,
 FOREIGN KEY(class_id) REFERENCES classes(id), FOREIGN KEY(created_by) REFERENCES users(id),
 CHECK(source_kind IN ('mistake','favorite')), CHECK(level BETWEEN 0 AND 5)
);
CREATE TABLE IF NOT EXISTS review_completions (
 id BIGINT AUTO_INCREMENT PRIMARY KEY,
 card_id BIGINT NOT NULL, student_id BIGINT NOT NULL, class_id BIGINT NOT NULL, created_by BIGINT NOT NULL,
 client_key VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 input_hash CHAR(64) CHARACTER SET ascii NOT NULL,
 feedback VARCHAR(8) NOT NULL, previous_level INT NOT NULL, level INT NOT NULL,
 due_at DATETIME(6) NOT NULL, completed_at DATETIME(6) NOT NULL,
 card_json JSON NOT NULL, intervals_json JSON NOT NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 UNIQUE KEY uq_review_completion(student_id,class_id,client_key),
 INDEX idx_review_history(student_id,class_id,id),
 FOREIGN KEY(card_id) REFERENCES review_cards(id) ON DELETE CASCADE,
 FOREIGN KEY(student_id) REFERENCES users(id) ON DELETE CASCADE,
 FOREIGN KEY(class_id) REFERENCES classes(id), FOREIGN KEY(created_by) REFERENCES users(id),
 CHECK(feedback IN ('good','again')), CHECK(level BETWEEN 0 AND 5)
);
