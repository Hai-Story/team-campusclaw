package tutor

import (
	"strings"
	"testing"
)

func TestTemplateValidation(t *testing.T) {
	app := appDefinitions()[0]
	for _, s := range []string{"新模板 {{topic}}；{{attempt}}", app.UserTemplate} {
		if err := validateAppTemplate(app, s); err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range []string{"缺少字段 {{topic}}", "未知 {{topic}} {{attempt}} {{other}}", "{{topic}} {{attempt}} {{bad-key}}", strings.Repeat("字", 300) + "{{topic}} {{attempt}}", "{{topic}} {{topic}} {{topic}} {{attempt}}"} {
		if err := validateAppTemplate(app, s); err == nil {
			t.Fatal("accepted invalid template", s)
		}
	}
}
