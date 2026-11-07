package tutor

import (
	"campusclaw/backend/internal/auth"
	"context"
	"database/sql"
	"net/http"
	"regexp"
	"strings"
)

var placeholder = regexp.MustCompile(`\{\{([a-z_]+)\}\}`)

func validateAppTemplate(app AIApp, text string) error {
	fields := map[string]bool{}
	for _, f := range app.Fields {
		fields[f.Key] = false
	}
	for _, m := range placeholder.FindAllStringSubmatch(text, -1) {
		if _, ok := fields[m[1]]; !ok {
			return templateError("模板包含未知占位符：" + m[1])
		}
		fields[m[1]] = true
	}
	rest := placeholder.ReplaceAllString(text, "")
	if strings.ContainsAny(rest, "{}") {
		return templateError("占位符格式须为 {{字段名}}")
	}
	worst := map[string]string{}
	for _, f := range app.Fields {
		if !fields[f.Key] {
			return templateError("模板必须保留 {{" + f.Key + "}}")
		}
		worst[f.Key] = strings.Repeat("字", f.Max)
	}
	if _, err := renderApp(appWithTemplate(app, text), worst); err != nil {
		return templateError("模板展开最多500字，请缩短固定文字或减少重复占位符")
	}
	return nil
}

type templateError string

func (e templateError) Error() string              { return string(e) }
func appWithTemplate(app AIApp, text string) AIApp { app.UserTemplate = text; return app }
func (s *Service) effectiveTemplate(ctx context.Context, classID int64, app *AIApp) error {
	err := s.DB.QueryRowContext(ctx, `SELECT user_template,version FROM tutor_app_templates WHERE class_id=? AND app_key=?`, classID, app.Key).Scan(&app.UserTemplate, &app.TemplateVersion)
	if err == sql.ErrNoRows {
		return nil
	}
	return err
}

// @api {"method":"PUT","path":"/api/ai-apps/{key}/template","summary":"教师在AI应用页修改本班提问模板","auth":"teacher","status":200,"request":{"type":"object","required":["user_template","expected_version"],"properties":{"user_template":{"type":"string","maxLength":500},"expected_version":{"type":"integer","minimum":0}}},"response":{"$ref":"#/components/schemas/AIApp"},"errors":{"400":"占位符或长度非法","401":"未登录","403":"仅教师可修改","404":"应用不存在","409":"模板已更新，请重新加载","503":"服务暂时不可用"},"description":"仅更新当前会话班级的提问模板；必须保留本应用全部字段，启动应用使用最新值。version=0表示尚未覆盖默认模板。"}
func (s *Service) SaveAppTemplate(w http.ResponseWriter, r *http.Request) {
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
		Text    string `json:"user_template"`
		Version *int64 `json:"expected_version"`
	}
	if !body(w, r, &in) {
		return
	}
	in.Text = strings.TrimSpace(in.Text)
	if in.Version == nil || *in.Version < 0 || len([]rune(in.Text)) > 500 {
		fail(w, 400, "模板版本及长度非法")
		return
	}
	if err := validateAppTemplate(*app, in.Text); err != nil {
		fail(w, 400, err.Error())
		return
	}
	var res sql.Result
	var err error
	if *in.Version == 0 {
		res, err = s.DB.ExecContext(r.Context(), `INSERT INTO tutor_app_templates(class_id,app_key,user_template,version) VALUES(?,?,?,1)`, u.ClassID, app.Key, in.Text)
	} else {
		res, err = s.DB.ExecContext(r.Context(), `UPDATE tutor_app_templates SET user_template=?,version=version+1 WHERE class_id=? AND app_key=? AND version=?`, in.Text, u.ClassID, app.Key, *in.Version)
	}
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			fail(w, 409, "模板已更新，请重新加载")
		} else {
			dbFail(w)
		}
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(w, 409, "模板已更新，请重新加载")
		return
	}
	app.UserTemplate = in.Text
	app.TemplateVersion = *in.Version + 1
	jsonResponse(w, 200, app)
}
