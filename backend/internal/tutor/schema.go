package tutor

import "database/sql"

// Migrate adds teaching tables without altering the earlier placeholder tables.
func Migrate(d *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS tutor_app_templates (class_id BIGINT NOT NULL,app_key VARCHAR(32) NOT NULL,user_template TEXT NOT NULL,version BIGINT NOT NULL DEFAULT 1,PRIMARY KEY(class_id,app_key),FOREIGN KEY(class_id) REFERENCES classes(id))`,
		`CREATE TABLE IF NOT EXISTS tutor_skills (id BIGINT AUTO_INCREMENT PRIMARY KEY,class_id BIGINT NOT NULL,name VARCHAR(32) NOT NULL,trigger_text VARCHAR(120) NOT NULL,subject_text VARCHAR(32) NOT NULL DEFAULT '',description TEXT NOT NULL,tools_json JSON NOT NULL,source_text TEXT NOT NULL,enabled BOOLEAN NOT NULL,seed_key VARCHAR(64) NULL,UNIQUE KEY uq_skill_name(class_id,name),UNIQUE KEY uq_skill_seed(class_id,seed_key),FOREIGN KEY(class_id) REFERENCES classes(id))`,
		`CREATE TABLE IF NOT EXISTS tutor_agents (id BIGINT AUTO_INCREMENT PRIMARY KEY,class_id BIGINT NOT NULL,name VARCHAR(64) NOT NULL,prompt TEXT NOT NULL,enabled BOOLEAN NOT NULL DEFAULT TRUE,seed_key VARCHAR(64) NULL,UNIQUE KEY uq_agent_name(class_id,name),UNIQUE KEY uq_agent_seed(class_id,seed_key),FOREIGN KEY(class_id) REFERENCES classes(id))`,
		`CREATE TABLE IF NOT EXISTS tutor_agent_skills (agent_id BIGINT NOT NULL,skill_id BIGINT NOT NULL,PRIMARY KEY(agent_id,skill_id),FOREIGN KEY(agent_id) REFERENCES tutor_agents(id) ON DELETE CASCADE,FOREIGN KEY(skill_id) REFERENCES tutor_skills(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS tutor_agent_materials (agent_id BIGINT NOT NULL,material_id BIGINT NOT NULL,PRIMARY KEY(agent_id,material_id),FOREIGN KEY(agent_id) REFERENCES tutor_agents(id) ON DELETE CASCADE,FOREIGN KEY(material_id) REFERENCES materials(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS conversations (id BIGINT AUTO_INCREMENT PRIMARY KEY,class_id BIGINT NOT NULL,user_id BIGINT NOT NULL,agent_id BIGINT NOT NULL,title VARCHAR(24) NOT NULL DEFAULT '新的提问',created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,INDEX idx_conversation_owner(user_id,class_id,updated_at),FOREIGN KEY(agent_id) REFERENCES tutor_agents(id),FOREIGN KEY(user_id) REFERENCES users(id),FOREIGN KEY(class_id) REFERENCES classes(id))`,
		`CREATE TABLE IF NOT EXISTS messages (id BIGINT AUTO_INCREMENT PRIMARY KEY,conversation_id BIGINT NOT NULL,role VARCHAR(16) NOT NULL,content MEDIUMTEXT NOT NULL,citations_json JSON NOT NULL,skill_name VARCHAR(32) NOT NULL DEFAULT '',status VARCHAR(16) NOT NULL DEFAULT 'complete',created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,INDEX idx_message_conv(conversation_id,id),CONSTRAINT chk_message_role CHECK(role IN ('user','assistant')),FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE CASCADE)`,
	}
	for _, q := range stmts {
		if _, err := d.Exec(q); err != nil {
			return err
		}
	}
	var resourceCols int
	if err := d.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='tutor_skills' AND column_name='resources_json'`).Scan(&resourceCols); err != nil {
		return err
	}
	if resourceCols == 0 {
		if _, err := d.Exec(`ALTER TABLE tutor_skills ADD COLUMN resources_json JSON NULL`); err != nil {
			return err
		}
	}
	var traceCols int
	if err := d.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='messages' AND column_name='llm_trace_json'`).Scan(&traceCols); err != nil {
		return err
	}
	if traceCols == 0 {
		if _, err := d.Exec(`ALTER TABLE messages ADD COLUMN llm_trace_json JSON NULL`); err != nil {
			return err
		}
	}
	// Preserve historical agents while freeing names for active replacements.
	for _, col := range []struct{ name, definition string }{
		{"deleted_at", "TIMESTAMP NULL DEFAULT NULL"},
		{"active_name", "VARCHAR(64) GENERATED ALWAYS AS (CASE WHEN deleted_at IS NULL THEN name ELSE NULL END) STORED"},
	} {
		var n int
		if err := d.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='tutor_agents' AND column_name=?`, col.name).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			if _, err := d.Exec(`ALTER TABLE tutor_agents ADD COLUMN ` + col.name + ` ` + col.definition); err != nil {
				return err
			}
		}
	}
	var n int
	if err := d.QueryRow(`SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='tutor_agents' AND index_name='uq_agent_active_name'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := d.Exec(`ALTER TABLE tutor_agents ADD UNIQUE KEY uq_agent_active_name(class_id,active_name)`); err != nil {
			return err
		}
	}
	if err := d.QueryRow(`SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='tutor_agents' AND index_name='uq_agent_name'`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		if _, err := d.Exec(`ALTER TABLE tutor_agents DROP INDEX uq_agent_name`); err != nil {
			return err
		}
	}
	return nil
}

const seedSource = "名称：解题引导\n适用条件：方程,去括号,分数,通分,面积,应用题,未知数,怎么解,思路\n工具：knowledge_search\n启用：是\n\n先了解学生已经做到哪一步，指出错误与下一步检查方法，不直接给最终答案。优先引用已提供的本班资料。"

func Seed(d *sql.DB) error {
	rows, err := d.Query(`SELECT id FROM classes ORDER BY id`)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, classID := range ids {
		tx, err := d.Begin()
		if err != nil {
			return err
		}
		err = seedClass(tx, classID)
		if err != nil {
			tx.Rollback()
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
func seedClass(tx *sql.Tx, classID int64) error {
	sk, _ := ParseSkill(seedSource)
	if _, err := tx.Exec(`INSERT IGNORE INTO tutor_skills(class_id,name,trigger_text,subject_text,description,tools_json,source_text,enabled,seed_key) VALUES(?,?,?,?,?, ?,?,TRUE,'guidance')`, classID, sk.Name, sk.Trigger, sk.Subject, sk.Description, "[\"knowledge_search\"]", seedSource); err != nil {
		return err
	}
	var skillID int64
	if err := tx.QueryRow(`SELECT id FROM tutor_skills WHERE class_id=? AND seed_key='guidance'`, classID).Scan(&skillID); err != nil {
		return err
	}
	for i, name := range []string{"解题助手", "知识讲解助手"} {
		key := []string{"guidance", "explain"}[i]
		prompt := []string{"你是学生自学的解题助手。用简体中文，循序渐进，优先关注学生卡在哪一步。", "你是中小学知识讲解助手。用简体中文，清楚解释概念，优先使用给出的资料，不编造出处。"}[i]
		res, err := tx.Exec(`INSERT IGNORE INTO tutor_agents(class_id,name,prompt,enabled,seed_key) VALUES(?,?,?,TRUE,?)`, classID, name, prompt, key)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			continue
		} // Never overwrite teacher edits or bindings on restart.
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if i == 0 {
			if _, err = tx.Exec(`INSERT INTO tutor_agent_skills(agent_id,skill_id) VALUES(?,?)`, id, skillID); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(`INSERT INTO tutor_agent_materials(agent_id,material_id) SELECT ?,id FROM materials WHERE class_id=?`, id, classID); err != nil {
			return err
		}
	}
	return seedApps(tx, classID)
}
