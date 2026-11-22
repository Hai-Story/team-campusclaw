CREATE TABLE IF NOT EXISTS lesson_plans (
 id BIGINT AUTO_INCREMENT PRIMARY KEY,
 class_id BIGINT NOT NULL,
 created_by BIGINT NOT NULL,
 subject VARCHAR(16) NOT NULL,
 title VARCHAR(120) NOT NULL,
 input_json JSON NOT NULL,
 body MEDIUMTEXT NOT NULL,
 citations_json JSON NOT NULL,
 client_key VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 input_hash CHAR(64) CHARACTER SET ascii NOT NULL,
 version INT NOT NULL DEFAULT 1,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 UNIQUE KEY uq_lesson_request(class_id,created_by,client_key),
 INDEX idx_lesson_owner(class_id,created_by,subject,id),
 FOREIGN KEY(class_id) REFERENCES classes(id),
 FOREIGN KEY(created_by) REFERENCES users(id)
);
