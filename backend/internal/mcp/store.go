package mcp

import (
	"database/sql"
	"fmt"
)

func Migrate(d *sql.DB) error {
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS mcp_servers(id BIGINT AUTO_INCREMENT PRIMARY KEY,class_id BIGINT NOT NULL,name VARCHAR(48) NOT NULL,base_url VARCHAR(512) NOT NULL,token_cipher BLOB NOT NULL,connected BOOLEAN NOT NULL DEFAULT FALSE,last_error VARCHAR(255) NOT NULL DEFAULT '',revision BIGINT NOT NULL DEFAULT 0,created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,UNIQUE KEY uq_mcp_server(class_id,name),FOREIGN KEY(class_id) REFERENCES classes(id))`,
		`CREATE TABLE IF NOT EXISTS mcp_tools(server_id BIGINT NOT NULL,name VARCHAR(48) NOT NULL,description TEXT NOT NULL,input_schema JSON NOT NULL,annotations JSON NOT NULL,fingerprint CHAR(64) NOT NULL,approved BOOLEAN NOT NULL DEFAULT FALSE,PRIMARY KEY(server_id,name),FOREIGN KEY(server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS mcp_tool_reviews(server_id BIGINT NOT NULL,name VARCHAR(48) NOT NULL,last_fingerprint CHAR(64) NOT NULL,approved_fingerprint CHAR(64) NOT NULL DEFAULT '',PRIMARY KEY(server_id,name),FOREIGN KEY(server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS mcp_tool_changes(id BIGINT AUTO_INCREMENT PRIMARY KEY,class_id BIGINT NOT NULL,server_id BIGINT NOT NULL,tool_name VARCHAR(48) NOT NULL,old_fingerprint CHAR(64) NOT NULL,new_fingerprint CHAR(64) NOT NULL,created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,INDEX idx_mcp_changes(class_id,server_id))`,
		`CREATE TABLE IF NOT EXISTS mcp_tool_calls(id BIGINT AUTO_INCREMENT PRIMARY KEY,class_id BIGINT NOT NULL,server_id BIGINT NOT NULL,server_name VARCHAR(48) NOT NULL,tool_name VARCHAR(48) NOT NULL,tool_ref VARCHAR(100) NOT NULL,user_id BIGINT NOT NULL,conversation_id BIGINT NOT NULL,request_message_id BIGINT NOT NULL,round_number INT NOT NULL,status VARCHAR(16) NOT NULL,summary TEXT NOT NULL,created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,INDEX idx_mcp_audit(class_id,id),INDEX idx_mcp_budget(conversation_id,id))`,
	} {
		if _, e := d.Exec(q); e != nil {
			return fmt.Errorf("MCP建表失败: %w", e)
		}
	}
	var n int
	if e := d.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='messages' AND column_name='tool_calls_json'`).Scan(&n); e != nil {
		return e
	}
	if n == 0 {
		_, e := d.Exec(`ALTER TABLE messages ADD COLUMN tool_calls_json JSON NULL`)
		return e
	}
	return nil
}
