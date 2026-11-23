CREATE TABLE IF NOT EXISTS knowledge_points (
 id BIGINT AUTO_INCREMENT PRIMARY KEY,
 class_id BIGINT NOT NULL,
 subject VARCHAR(16) NOT NULL,
 name VARCHAR(120) NOT NULL,
 chapter VARCHAR(120) NOT NULL DEFAULT '',
 description TEXT NOT NULL,
 version INT NOT NULL DEFAULT 1,
 created_by BIGINT NOT NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 UNIQUE KEY uq_point_name(class_id,subject,name),
 INDEX idx_point_class(class_id,subject,id),
 FOREIGN KEY(class_id) REFERENCES classes(id),
 FOREIGN KEY(created_by) REFERENCES users(id)
);
CREATE TABLE IF NOT EXISTS knowledge_point_materials (
 point_id BIGINT NOT NULL, material_id BIGINT NOT NULL,
 PRIMARY KEY(point_id,material_id),
 FOREIGN KEY(point_id) REFERENCES knowledge_points(id) ON DELETE CASCADE,
 FOREIGN KEY(material_id) REFERENCES materials(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS knowledge_point_assignments (
 point_id BIGINT NOT NULL, assignment_id BIGINT NOT NULL,
 PRIMARY KEY(point_id,assignment_id),
 FOREIGN KEY(point_id) REFERENCES knowledge_points(id) ON DELETE CASCADE,
 FOREIGN KEY(assignment_id) REFERENCES grade_assignments(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS knowledge_favorites (
 point_id BIGINT NOT NULL, student_id BIGINT NOT NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 PRIMARY KEY(point_id,student_id),
 FOREIGN KEY(point_id) REFERENCES knowledge_points(id) ON DELETE CASCADE,
 FOREIGN KEY(student_id) REFERENCES users(id) ON DELETE CASCADE
);
