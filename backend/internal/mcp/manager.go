package mcp

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

type Server struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	Connected bool   `json:"connected"`
	HasAuth   bool   `json:"has_auth"`
	LastError string `json:"last_error"`
	ToolCount int    `json:"tool_count"`
	Revision  int64  `json:"-"`
	cipher    []byte
}
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
	Annotations map[string]any  `json:"annotations"`
	Fingerprint string          `json:"fingerprint"`
	Approved    bool            `json:"approved"`
	ReadOnly    bool            `json:"read_only"`
}
type Input struct {
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	AuthToken string `json:"auth_token"`
}
type Manager struct {
	DB                                        *sql.DB
	key                                       []byte
	DefaultToken                              string
	allowed                                   map[string]bool
	locks                                     sync.Map
	MaxRounds, MaxPerTurn, MaxPerConversation int
}
type apiError struct {
	Code    int
	Message string
}

func (e *apiError) Error() string { return e.Message }
func invalid(s string) error      { return &apiError{400, s} }
func New(d *sql.DB, keyHex, token, urls string) (*Manager, error) {
	key, e := hex.DecodeString(keyHex)
	if e != nil || len(key) != 32 {
		return nil, errors.New("MCP_ENCRYPTION_KEY须为64位十六进制")
	}
	if len(token) < 24 {
		return nil, errors.New("MCP_AUTH_TOKEN至少24字符")
	}
	m := &Manager{DB: d, key: key, DefaultToken: token, allowed: map[string]bool{}, MaxRounds: 3, MaxPerTurn: 6, MaxPerConversation: 30}
	for _, s := range strings.Split(urls, ",") {
		u, e := normalizeURL(strings.TrimSpace(s))
		if e != nil {
			return nil, e
		}
		m.allowed[u] = true
	}
	return m, nil
}
func (m *Manager) lock(id int64) *sync.RWMutex {
	v, _ := m.locks.LoadOrStore(id, &sync.RWMutex{})
	return v.(*sync.RWMutex)
}
func (m *Manager) Get(ctx context.Context, class, id int64) (Server, error) {
	var s Server
	var n int
	e := m.DB.QueryRowContext(ctx, `SELECT s.id,s.name,s.base_url,s.token_cipher,s.connected,s.last_error,s.revision,(SELECT COUNT(*) FROM mcp_tools t WHERE t.server_id=s.id) FROM mcp_servers s WHERE s.id=? AND s.class_id=?`, id, class).Scan(&s.ID, &s.Name, &s.BaseURL, &s.cipher, &s.Connected, &s.LastError, &s.Revision, &n)
	s.HasAuth = len(s.cipher) > 0
	s.ToolCount = n
	return s, e
}
func (m *Manager) List(ctx context.Context, class int64) ([]Server, error) {
	rows, e := m.DB.QueryContext(ctx, `SELECT id FROM mcp_servers WHERE class_id=? ORDER BY id`, class)
	if e != nil {
		return nil, e
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return nil, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	out := []Server{}
	for _, id := range ids {
		s, e := m.Get(ctx, class, id)
		if e != nil {
			return nil, e
		}
		out = append(out, s)
	}
	return out, nil
}
func (m *Manager) Save(ctx context.Context, class, id int64, in Input) (Server, error) {
	l := m.lock(id)
	l.Lock()
	defer l.Unlock()
	var s Server
	var e error
	if id > 0 {
		s, e = m.Get(ctx, class, id)
		if e != nil {
			return s, e
		}
	}
	in.Name = strings.TrimSpace(in.Name)
	if !serverName.MatchString(in.Name) {
		return s, invalid("服务器名称须以小写字母开头，最多48位，仅含小写字母、数字、下划线或连字符")
	}
	base, e := normalizeURL(strings.TrimSpace(in.BaseURL))
	if e != nil {
		return s, invalid(e.Error())
	}
	if !m.allowed[base] {
		return s, invalid("Base URL未列入部署环境MCP_ALLOWED_BASE_URLS白名单")
	}
	if in.AuthToken != "" {
		if len(in.AuthToken) > 512 || strings.ContainsAny(in.AuthToken, "\r\n\t ") {
			return s, invalid("Bearer令牌须为不含空白的字符串，最多512字符")
		}
		s.cipher, e = encrypt(m.key, in.AuthToken)
	} else if id == 0 {
		if base != "http://mcp-demo:8090" {
			return s, invalid("新服务器须填写令牌")
		}
		s.cipher, e = encrypt(m.key, m.DefaultToken)
	}
	if e != nil {
		return s, e
	}
	tx, e := m.DB.BeginTx(ctx, nil)
	if e != nil {
		return s, e
	}
	defer tx.Rollback()
	if id == 0 {
		var r sql.Result
		r, e = tx.ExecContext(ctx, `INSERT INTO mcp_servers(class_id,name,base_url,token_cipher) VALUES(?,?,?,?)`, class, in.Name, base, s.cipher)
		if e == nil {
			id, e = r.LastInsertId()
		}
	} else {
		_, e = tx.ExecContext(ctx, `UPDATE mcp_servers SET name=?,base_url=?,token_cipher=?,connected=FALSE,last_error='',revision=revision+1 WHERE id=? AND class_id=?`, in.Name, base, s.cipher, id, class)
		if e == nil {
			_, e = tx.ExecContext(ctx, `DELETE FROM mcp_tools WHERE server_id=?`, id)
		}
	}
	if e != nil {
		if strings.Contains(e.Error(), "Duplicate entry") {
			return s, invalid("本班服务器名称已存在")
		}
		return s, e
	}
	if e = tx.Commit(); e != nil {
		return s, e
	}
	return m.Get(ctx, class, id)
}
func (m *Manager) Disconnect(ctx context.Context, class, id int64) error {
	l := m.lock(id)
	l.Lock()
	defer l.Unlock()
	if _, e := m.Get(ctx, class, id); e != nil {
		return e
	}
	return m.clear(ctx, id, "")
}
func (m *Manager) clear(ctx context.Context, id int64, reason string) error {
	tx, e := m.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `UPDATE mcp_servers SET connected=FALSE,last_error=?,revision=revision+1 WHERE id=?`, reason, id); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `DELETE FROM mcp_tools WHERE server_id=?`, id); e != nil {
		return e
	}
	return tx.Commit()
}
func (m *Manager) Delete(ctx context.Context, class, id int64) error {
	l := m.lock(id)
	l.Lock()
	defer l.Unlock()
	if _, e := m.Get(ctx, class, id); e != nil {
		return e
	}
	_, e := m.DB.ExecContext(ctx, `DELETE FROM mcp_servers WHERE id=? AND class_id=?`, id, class)
	return e
}
func (m *Manager) Tools(ctx context.Context, class, id int64) ([]Tool, error) {
	if _, e := m.Get(ctx, class, id); e != nil {
		return nil, e
	}
	return m.tools(ctx, id)
}
func (m *Manager) tools(ctx context.Context, id int64) ([]Tool, error) {
	rows, e := m.DB.QueryContext(ctx, `SELECT name,description,input_schema,annotations,fingerprint,approved FROM mcp_tools WHERE server_id=? ORDER BY name`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Tool{}
	for rows.Next() {
		var t Tool
		var raw, ann []byte
		if e = rows.Scan(&t.Name, &t.Description, &raw, &ann, &t.Fingerprint, &t.Approved); e != nil {
			return nil, e
		}
		t.InputSchema = raw
		if json.Unmarshal(ann, &t.Annotations) != nil {
			return nil, errors.New("缓存数据无效")
		}
		t.ReadOnly = readOnly(t)
		out = append(out, t)
	}
	return out, rows.Err()
}
func (m *Manager) Connect(ctx context.Context, class, id int64) (Server, error) {
	l := m.lock(id)
	l.Lock()
	defer l.Unlock()
	s, e := m.Get(ctx, class, id)
	if e != nil {
		return s, e
	}
	if e = m.clear(ctx, id, ""); e != nil {
		return s, e
	}
	var tx *sql.Tx
	failed := func(e error) (Server, error) {
		if tx != nil {
			_ = tx.Rollback()
		}
		if clearErr := m.clear(ctx, id, e.Error()); clearErr != nil {
			return s, clearErr
		}
		return s, &apiError{502, e.Error()}
	}
	if !m.allowed[s.BaseURL] {
		return failed(errors.New("服务器地址已移出部署白名单"))
	}
	token, e := decrypt(m.key, s.cipher)
	if e != nil {
		return failed(e)
	}
	var init struct {
		Version      string                     `json:"protocolVersion"`
		Capabilities map[string]json.RawMessage `json:"capabilities"`
	}
	if e = rpc(ctx, s.BaseURL, token, "initialize", map[string]any{"protocolVersion": Protocol, "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "CampusClaw", "version": "0.4.0"}}, &init); e != nil {
		return failed(e)
	}
	if init.Version != Protocol || init.Capabilities["tools"] == nil {
		return failed(errors.New("MCP版本或工具能力协商失败"))
	}
	if e = rpc(ctx, s.BaseURL, token, "notifications/initialized", map[string]any{}, nil); e != nil {
		return failed(e)
	}
	var discovery struct {
		Tools      []Tool `json:"tools"`
		NextCursor string `json:"nextCursor"`
	}
	if e = rpc(ctx, s.BaseURL, token, "tools/list", map[string]any{}, &discovery); e != nil {
		return failed(e)
	}
	if len(discovery.Tools) > 64 || discovery.NextCursor != "" {
		return failed(errors.New("本课仅接受单页最多64个工具"))
	}
	seen := map[string]bool{}
	for i := range discovery.Tools {
		t := &discovery.Tools[i]
		if !toolName.MatchString(t.Name) || seen[t.Name] || len([]rune(t.Description)) > 2000 {
			return failed(errors.New("工具名称、说明长度或唯一性校验失败"))
		}
		seen[t.Name] = true
		if _, e = compileSchema(t.InputSchema); e != nil {
			return failed(e)
		}
		t.Fingerprint = fingerprint(*t)
	}
	tx, e = m.DB.BeginTx(ctx, nil)
	if e != nil {
		return failed(errors.New("发现结果保存失败"))
	}
	defer tx.Rollback()
	for _, t := range discovery.Tools {
		var old, approved string
		e = tx.QueryRowContext(ctx, `SELECT last_fingerprint,approved_fingerprint FROM mcp_tool_reviews WHERE server_id=? AND name=?`, id, t.Name).Scan(&old, &approved)
		if e != nil && e != sql.ErrNoRows {
			return failed(errors.New("发现结果保存失败"))
		}
		if old != "" && old != t.Fingerprint {
			if _, e = tx.ExecContext(ctx, `INSERT INTO mcp_tool_changes(class_id,server_id,tool_name,old_fingerprint,new_fingerprint) VALUES(?,?,?,?,?)`, class, id, t.Name, old, t.Fingerprint); e != nil {
				return failed(errors.New("说明变更记录失败"))
			}
			approved = ""
		}
		ann, _ := json.Marshal(t.Annotations)
		if _, e = tx.ExecContext(ctx, `INSERT INTO mcp_tools(server_id,name,description,input_schema,annotations,fingerprint,approved) VALUES(?,?,?,?,?,?,?)`, id, t.Name, t.Description, string(t.InputSchema), string(ann), t.Fingerprint, approved == t.Fingerprint); e != nil {
			return failed(errors.New("发现结果保存失败"))
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO mcp_tool_reviews(server_id,name,last_fingerprint,approved_fingerprint) VALUES(?,?,?,?) ON DUPLICATE KEY UPDATE last_fingerprint=VALUES(last_fingerprint),approved_fingerprint=VALUES(approved_fingerprint)`, id, t.Name, t.Fingerprint, approved); e != nil {
			return failed(errors.New("发现结果保存失败"))
		}
	}
	if _, e = tx.ExecContext(ctx, `UPDATE mcp_servers SET connected=TRUE,last_error='',revision=revision+1 WHERE id=?`, id); e != nil {
		return failed(errors.New("连接状态保存失败"))
	}
	if e = tx.Commit(); e != nil {
		return failed(errors.New("发现结果保存失败"))
	}
	return m.Get(ctx, class, id)
}
func (m *Manager) Approve(ctx context.Context, class, id int64, name, hash string) error {
	l := m.lock(id)
	l.Lock()
	defer l.Unlock()
	s, e := m.Get(ctx, class, id)
	if e != nil {
		return e
	}
	if !s.Connected {
		return invalid("请先连接服务器")
	}
	ts, e := m.tools(ctx, id)
	if e != nil {
		return e
	}
	for _, t := range ts {
		if t.Name != name {
			continue
		}
		if t.Fingerprint != hash {
			return &apiError{409, "工具说明已变化，请重新查看并确认"}
		}
		if !readOnly(t) {
			return invalid("本课只允许只读、非外发且非破坏性的工具")
		}
		tx, e := m.DB.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		defer tx.Rollback()
		if _, e = tx.ExecContext(ctx, `UPDATE mcp_tools SET approved=TRUE WHERE server_id=? AND name=?`, id, name); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `UPDATE mcp_tool_reviews SET approved_fingerprint=? WHERE server_id=? AND name=?`, hash, id, name); e != nil {
			return e
		}
		return tx.Commit()
	}
	return sql.ErrNoRows
}
func (m *Manager) ValidateRefs(ctx context.Context, class int64, refs []string) error {
	for _, ref := range refs {
		if ref == "knowledge_search" {
			continue
		}
		name, server, e := ParseRef(ref)
		if e != nil {
			return e
		}
		var id int64
		if e = m.DB.QueryRowContext(ctx, `SELECT id FROM mcp_servers WHERE class_id=? AND name=? AND connected=TRUE`, class, server).Scan(&id); e != nil {
			if e == sql.ErrNoRows {
				return invalid("MCP服务器不存在或未连接")
			}
			return e
		}
		ts, e := m.tools(ctx, id)
		if e != nil {
			return e
		}
		found := false
		for _, t := range ts {
			if t.Name == name && t.Approved && readOnly(t) {
				found = true
			}
		}
		if !found {
			return invalid(fmt.Sprintf("工具%s未发现、未批准或不允许执行", ref))
		}
	}
	return nil
}
