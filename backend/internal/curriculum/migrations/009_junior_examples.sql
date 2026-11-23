CREATE TABLE IF NOT EXISTS curriculum_seed_items (
 class_id BIGINT NOT NULL, created_by BIGINT NOT NULL,
 batch VARCHAR(32) NOT NULL, unit_key VARCHAR(64) NOT NULL,
 material_id BIGINT NOT NULL, entry_id BIGINT NOT NULL,
 status VARCHAR(16) NOT NULL DEFAULT 'staged', resources_json JSON NOT NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 PRIMARY KEY(class_id,created_by,batch,unit_key),
 FOREIGN KEY(class_id) REFERENCES classes(id), FOREIGN KEY(created_by) REFERENCES users(id),
 FOREIGN KEY(material_id) REFERENCES materials(id) ON DELETE CASCADE
);
