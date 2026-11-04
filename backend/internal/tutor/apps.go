package tutor

import (
	"campusclaw/backend/internal/auth"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

//go:embed builtins/*/SKILL.md
var builtinSkills embed.FS

type AppField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Placeholder string `json:"placeholder"`
	Max         int    `json:"max_length"`
}
type AIApp struct {
	TemplateVersion int64      `json:"template_version"`
	Key             string     `json:"key"`
	Name            string     `json:"name"`
	Description     string     `json:"description"`
	Icon            string     `json:"icon"`
	SystemTemplate  string     `json:"system_template"`
	UserTemplate    string     `json:"user_template"`
	Fields          []AppField `json:"fields"`
	AgentID         int64      `json:"agent_id"`
	Enabled         bool       `json:"enabled"`
}

func appDefinitions() []AIApp {
	return []AIApp{
		{Key: "concept", Name: "概念讲解", Icon: "spark", Description: "把抽象概念拆成定义、例子和自测问题。", SystemTemplate: "你是概念学习教练。用简体中文，按定义、直观例子、常见混淆、自测问题组织回答。根据学生程度解释；引用本班资料时给出编号，不编造依据。", UserTemplate: "请帮我理解概念：{{topic}}\n我目前的理解或困惑：{{attempt}}\n请给一个直观例子，再用一个问题检查我的理解。", Fields: []AppField{{"topic", "想理解的概念", "例如：分数通分为什么不改变大小", 120}, {"attempt", "已有理解或困惑", "说说你目前怎么理解", 180}}},
		{Key: "reading", Name: "阅读分析", Icon: "book", Description: "从原文线索出发，分析段落、主旨和表达作用。", SystemTemplate: "你是阅读分析教练。区分原文事实与推断，按文本线索、推理过程、可尝试的回答组织回应。不捏造原文，不代替学生完成整份阅读作业。", UserTemplate: "请帮我进行阅读分析。\n原文片段：{{text}}\n我的问题与尝试：{{attempt}}\n请先定位线索，再引导我归纳。", Fields: []AppField{{"text", "原文片段", "粘贴想讨论的一小段原文", 180}, {"attempt", "问题与尝试", "例如：这一句有什么作用？我认为…", 180}}},
		{Key: "writing", Name: "写作指导", Icon: "edit", Description: "保留自己的表达，获得具体且有理由的修改建议。", SystemTemplate: "你是写作教练。根据写作目标反馈结构、细节与表达，先肯定具体有效之处，再提出一个主要改进点与局部示例。保留作者事实和原意，不生成可直接代交的完整作文。", UserTemplate: "请给我写作指导。\n我的草稿片段：{{text}}\n写作目标或希望改进的地方：{{goal}}\n请保留我的原意，解释建议的理由。", Fields: []AppField{{"text", "草稿片段", "粘贴自己写的一小段文字", 180}, {"goal", "写作目标", "例如：想让这个场景更具体", 120}}},
		{Key: "review", Name: "错题复盘", Icon: "search", Description: "找出第一处偏差，形成下次可用的检查方法。", SystemTemplate: "你是错题复盘教练。根据题目与学生作答定位首个错误，说明错因、下一步检查方法与巩固建议。信息不足先追问，不虚构正式分数，不直接给最终答案。", UserTemplate: "请帮我复盘这道错题。\n题目：{{text}}\n我的作答与已知错误：{{attempt}}\n请指出首个需要检查的地方和下一步思路，不直接给最终答案。", Fields: []AppField{{"text", "题目", "输入文字题目，暂不支持图片", 180}, {"attempt", "我的作答与错误", "写下自己的步骤或老师指出的问题", 180}}},
	}
}
func renderApp(app AIApp, fields map[string]string) (string, error) {
	if len(fields) != len(app.Fields) {
		return "", fmt.Errorf("请填写应用要求的全部字段")
	}
	pairs := []string{}
	for _, f := range app.Fields {
		v := strings.TrimSpace(fields[f.Key])
		if v == "" || len([]rune(v)) > f.Max {
			return "", fmt.Errorf("%s必填，最多%d字", f.Label, f.Max)
		}
		pairs = append(pairs, "{{"+f.Key+"}}", v)
	}
	q := strings.NewReplacer(pairs...).Replace(app.UserTemplate)
	if len([]rune(q)) > 500 {
		return "", fmt.Errorf("展开后的提问超过500字，请缩短输入")
	}
	return q, nil
}
func seedApps(tx *sql.Tx, classID int64) error {
	for _, app := range appDefinitions() {
		source, err := builtinSkills.ReadFile("builtins/" + app.Key + "/SKILL.md")
		if err != nil {
			return err
		}
		sk, err := ParseSkill(string(source))
		if err != nil {
			return err
		}
		tools, _ := json.Marshal(sk.Tools)
		key := "app-" + app.Key
		if _, err = tx.Exec(`INSERT IGNORE INTO tutor_skills(class_id,name,trigger_text,subject_text,description,tools_json,source_text,enabled,seed_key) VALUES(?,?,?,?,?,?,?,TRUE,?)`, classID, sk.Name, sk.Trigger, sk.Subject, sk.Description, string(tools), sk.Source, key); err != nil {
			return err
		}

		// Upgrade only the exact v1 generated source; preserve every teacher edit and binding.
		prior := strings.Replace(sk.Source, "subject: "+sk.Subject+"\n", "", 1)
		prior = strings.Replace(prior, "局部示例中新增的细节须标为假设，并请学生替换为亲身观察；不得称这些假设为学生的真实经历。", "", 1)
		if prior != sk.Source {
			if _, err = tx.Exec(`UPDATE tutor_skills SET subject_text=?,description=?,source_text=? WHERE class_id=? AND seed_key=? AND source_text=?`, sk.Subject, sk.Description, sk.Source, classID, key, prior); err != nil {
				return err
			}
		}
		var sid int64
		if err = tx.QueryRow(`SELECT id FROM tutor_skills WHERE class_id=? AND (seed_key=? OR name=?) ORDER BY seed_key IS NULL LIMIT 1`, classID, key, sk.Name).Scan(&sid); err != nil {
			return err
		}
		res, err := tx.Exec(`INSERT IGNORE INTO tutor_agents(class_id,name,prompt,enabled,seed_key) VALUES(?,?,?,TRUE,?)`, classID, app.Name, app.SystemTemplate, key)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			continue
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO tutor_agent_skills(agent_id,skill_id) VALUES(?,?)`, id, sid); err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO tutor_agent_materials(agent_id,material_id) SELECT ?,id FROM materials WHERE class_id=?`, id, classID); err != nil {
			return err
		}
	}
	return nil
}

// @api {"method":"GET","path":"/api/ai-apps","summary":"本班内置AI应用及对应提示词模板","auth":"login","status":200,"request":null,"response":{"type":"array","items":{"$ref":"#/components/schemas/AIApp"}},"errors":{"401":"未登录","503":"服务暂时不可用"}}
func (s *Service) Apps(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromContext(r.Context())
	apps := appDefinitions()
	for i := range apps {
		if err := s.effectiveTemplate(r.Context(), u.ClassID, &apps[i]); err != nil {
			dbFail(w)
			return
		}
		err := s.DB.QueryRowContext(r.Context(), `SELECT id,enabled FROM tutor_agents WHERE class_id=? AND seed_key=? AND deleted_at IS NULL`, u.ClassID, "app-"+apps[i].Key).Scan(&apps[i].AgentID, &apps[i].Enabled)
		if err != nil && err != sql.ErrNoRows {
			dbFail(w)
			return
		}
	}
	jsonResponse(w, 200, apps)
}

// @api {"method":"POST","path":"/api/ai-apps/{key}/launch","summary":"展开应用模板并创建本人独立会话","auth":"student","status":201,"request":{"type":"object","required":["fields"],"properties":{"fields":{"type":"object","additionalProperties":{"type":"string","maxLength":180}}}},"response":{"type":"object","properties":{"conversation":{"$ref":"#/components/schemas/Conversation"},"prompt":{"type":"string","maxLength":500}}},"errors":{"400":"字段缺失、超限或应用已停用","401":"未登录","403":"仅学生可启动","404":"应用不存在或已删除","503":"服务暂时不可用"},"description":"仅展开提问，不自动发送LLM；用户可编辑后确认发送。"}
func (s *Service) LaunchApp(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromContext(r.Context())
	var app *AIApp
	for _, v := range appDefinitions() {
		if v.Key == r.PathValue("key") {
			copy := v
			app = &copy
			break
		}
	}
	if app == nil {
		missing(w)
		return
	}
	var in struct {
		Fields map[string]string `json:"fields"`
	}
	if !body(w, r, &in) {
		return
	}
	if err := s.effectiveTemplate(r.Context(), u.ClassID, app); err != nil {
		dbFail(w)
		return
	}
	prompt, err := renderApp(*app, in.Fields)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	var aid int64
	var enabled bool
	err = s.DB.QueryRowContext(r.Context(), `SELECT id,enabled FROM tutor_agents WHERE class_id=? AND seed_key=? AND deleted_at IS NULL`, u.ClassID, "app-"+app.Key).Scan(&aid, &enabled)
	if err != nil {
		resourceError(w, err)
		return
	}
	gate := s.agentGate(aid)
	gate.RLock()
	defer gate.RUnlock()
	a, err := s.agent(r.Context(), aid, u.ClassID, false)
	if err != nil {
		resourceError(w, err)
		return
	}
	if !enabled || !a.Enabled {
		fail(w, 400, "该应用已停用，请联系教师")
		return
	}
	res, err := s.DB.ExecContext(r.Context(), `INSERT INTO conversations(class_id,user_id,agent_id) VALUES(?,?,?)`, u.ClassID, u.ID, aid)
	if err != nil {
		dbFail(w)
		return
	}
	id, err := res.LastInsertId()
	if err != nil {
		dbFail(w)
		return
	}
	c, err := s.conversation(r.Context(), id, u)
	if err != nil {
		dbFail(w)
		return
	}
	jsonResponse(w, 201, map[string]any{"conversation": c, "prompt": prompt})
}
