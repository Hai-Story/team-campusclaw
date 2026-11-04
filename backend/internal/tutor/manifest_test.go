package tutor

import (
	"archive/zip"
	"bytes"
	"campusclaw/backend/internal/retrieval"
	"strings"
	"testing"
)

func skillZip(files map[string]string) []byte {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for n, s := range files {
		f, _ := z.Create(n)
		f.Write([]byte(s))
	}
	z.Close()
	return b.Bytes()
}

const manifestFixture = "---\nname: 测试技能\ndescription: 测试渐进加载\ntriggers: [面积, 方程]\ntools: [knowledge_search]\nenabled: true\n---\n\nACTIVATED_BODY 只引导下一步。"

func TestManifestPackage(t *testing.T) {
	source, resources, err := readSkillUpload("guide.zip", skillZip(map[string]string{"guide/SKILL.md": manifestFixture, "guide/references/note.md": "RESOURCE_NOT_INJECTED"}))
	if err != nil {
		t.Fatal(err)
	}
	sk, err := ParseSkill(source)
	if err != nil {
		t.Fatal(err)
	}
	sk.ID = 1
	sk.Resources = resources
	if sk.Name != "测试技能" || len(resources) != 1 || len(sk.Tools) != 1 {
		t.Fatalf("%+v", sk)
	}
	inactive := Skill{ID: 2, Name: "未选中", Description: "INACTIVE_BODY", Enabled: true, Trigger: "阅读"}
	chosen := Route([]Skill{sk, inactive}, "面积如何检查")
	messages, _ := BuildMessages(Agent{Prompt: "ROLE_TEMPLATE"}, chosen, []Message{{Role: "user", Content: "面积如何检查"}}, []retrieval.Hit{})
	text := messages[0].Content
	if !strings.Contains(text, "ACTIVATED_BODY") || strings.Contains(text, "RESOURCE_NOT_INJECTED") || strings.Contains(text, "INACTIVE_BODY") {
		t.Fatal(text)
	}
	if Route([]Skill{sk}, "无触发") == nil {
	} else {
		t.Fatal("unexpected routing")
	}
}
func TestRejectUnsafeSkillPackages(t *testing.T) {
	for _, files := range []map[string]string{{"note.md": "missing"}, {"SKILL.md": manifestFixture, "../bad.txt": "x"}, {"SKILL.md": manifestFixture, "run.py": "print(1)"}, {"SKILL.md": manifestFixture, "references/b.txt": string([]byte{0xff})}, {"SKILL.md": manifestFixture, "a/SKILL.md": manifestFixture}, {"SKILL.md": manifestFixture, "references/huge.txt": strings.Repeat("x", 33000)}} {
		if _, _, err := readSkillUpload("x.zip", skillZip(files)); err == nil {
			t.Fatal("accepted unsafe package")
		}
	}
	if _, err := ParseSkill("---\nname: x\n---\nbody"); err == nil {
		t.Fatal("missing description accepted")
	}
}

func TestBuiltinEvidenceSubjectBounds(t *testing.T) {
	for _, key := range []string{"reading", "writing", "review"} {
		source, _ := builtinSkills.ReadFile("builtins/" + key + "/SKILL.md")
		sk, err := ParseSkill(string(source))
		if err != nil || sk.Subject == "" {
			t.Fatal(key, err)
		}
		msg, cites := BuildMessages(Agent{Prompt: "role"}, &sk, []Message{{Role: "user", Content: "问题"}}, []retrieval.Hit{{MaterialTitle: "科学笔记", Excerpt: "IRRELEVANT_SCIENCE"}, {MaterialTitle: sk.Subject + "笔记", Excerpt: "RELEVANT_EVIDENCE"}})
		if len(cites) != 1 || strings.Contains(msg[len(msg)-1].Content, "IRRELEVANT_SCIENCE") || !strings.Contains(msg[len(msg)-1].Content, "RELEVANT_EVIDENCE") {
			t.Fatal(key, cites)
		}
	}
}
