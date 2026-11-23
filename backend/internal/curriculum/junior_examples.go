package curriculum

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"campusclaw/backend/internal/auth"
	"campusclaw/backend/internal/chunking"
	"campusclaw/backend/internal/retrieval"
	"campusclaw/backend/internal/subjects"
)

//go:embed examples/junior.json
var juniorCatalog []byte

const JuniorBatch = "junior-v1"

type JuniorUnit struct {
	Key            string   `json:"key"`
	Subject        string   `json:"subject"`
	Grade          int      `json:"grade"`
	Title          string   `json:"title"`
	Points         []string `json:"points"`
	Facts          []string `json:"facts"`
	Misconceptions []string `json:"misconceptions"`
	Question       string   `json:"question"`
	Answer         string   `json:"answer"`
}

func JuniorUnits() ([]JuniorUnit, error) {
	var units []JuniorUnit
	if e := json.Unmarshal(juniorCatalog, &units); e != nil {
		return nil, e
	}
	seen := map[string]bool{}
	counts := map[string]int{}
	for _, u := range units {
		if !ValidSubject(u.Subject) || u.Grade < 7 || u.Grade > 9 || len(u.Points) != 2 || len(u.Facts) != 3 || len(u.Misconceptions) != 3 || u.Question == "" || u.Answer == "" || seen[u.Key] {
			return nil, ErrInvalid
		}
		seen[u.Key] = true
		counts[u.Subject]++
	}
	if len(counts) != 9 {
		return nil, ErrInvalid
	}
	for _, n := range counts {
		if n != 6 {
			return nil, ErrInvalid
		}
	}
	return units, nil
}
func (u JuniorUnit) chapter() string { return fmt.Sprintf("初%d · %s", u.Grade-6, u.Title) }
func (u JuniorUnit) lecture() string {
	return "# " + subjects.Name(u.Subject) + " · " + u.chapter() + "\n\n原创示例讲义，供练习与备课编辑。\n\n## 概念梳理\n\n- " + strings.Join(u.Facts, "\n- ") + "\n\n## 辨析与运用\n\n学习时先辨明" + u.Points[0] + "与" + u.Points[1] + "的联系，结合概念、条件和具体情境核对表述。\n\n## 练习思考\n\n" + u.Question + "\n\n请写出依据与过程，再核对定义、条件和结论是否一致。\n"
}
func (u JuniorUnit) questions(ids []int64, offset int) []QuestionItem {
	single := append([]string{u.Facts[0]}, u.Misconceptions...)
	multi := []string{u.Facts[1], u.Misconceptions[1], u.Facts[2], u.Misconceptions[2]}
	rotate := func(v []string) []string { k := offset % len(v); return append(append([]string{}, v[k:]...), v[:k]...) }
	s := rotate(single)
	m := rotate(multi)
	sa := []string{}
	ma := []string{}
	for i, v := range s {
		if v == u.Facts[0] {
			sa = append(sa, string(rune('A'+i)))
		}
	}
	for i, v := range m {
		if v == u.Facts[1] || v == u.Facts[2] {
			ma = append(ma, string(rune('A'+i)))
		}
	}
	stem := "【示例】" + u.Title
	tf, ta := u.Misconceptions[0], "false"
	if offset%2 == 1 {
		tf, ta = u.Facts[0], "true"
	}
	base := func(t, st string, options, answer []string, why string) QuestionItem {
		return QuestionItem{Type: t, Stem: st, Options: options, Answer: answer, Explanation: why, KnowledgePointIDs: ids, Difficulty: "medium", CitationIndices: []int{1}}
	}
	return []QuestionItem{
		base("single_choice", stem+"：下列说法正确的是？", s, sa, u.Facts[0]+"。请结合讲义中的概念和适用条件排除错误表述。[1]"),
		base("multiple_choice", stem+"：下列哪些表述正确？", m, ma, u.Facts[1]+"；"+u.Facts[2]+"。两项都符合概念与条件。[1]"),
		base("true_false", stem+"：判断以下表述是否正确——"+tf, []string{}, []string{ta}, u.Facts[0]+"。[1]"),
		base("short_answer", stem+"："+u.Question, []string{}, []string{u.Answer}, u.Answer+"[1]"),
	}
}
func (u JuniorUnit) lesson() LessonOutput {
	return LessonOutput{
		Goals:     []string{"说明" + u.Points[0] + "与" + u.Points[1] + "，能结合情境说明依据。", "完成练习并辨析常见误解。"},
		KeyPoints: []string{u.Facts[0], u.Facts[1]}, Difficulties: []string{"辨析误解：“" + u.Misconceptions[0] + "”，用条件和证据进行纠正。"},
		Activities: []LessonActivity{
			{"情境导入", 5, []string{"出示问题：" + u.Question, "收集已有认识，暂不公布结论。"}},
			{"概念梳理", 10, []string{"结合讲义[1]讨论：" + u.Facts[0], "说明：" + u.Facts[1]}},
			{"合作探究", 12, []string{"以" + u.Points[0] + "为起点，分析与" + u.Points[1] + "的联系。", "各组给出依据，检查：" + u.Facts[2]}},
			{"练习与反馈", 10, []string{u.Question, "比较不同过程，并对照概念核查，而非只看结论。"}},
			{"易错辨析", 5, []string{"辨析：“" + u.Misconceptions[0] + "”。", "明确正确认识：" + u.Facts[0]}},
			{"课堂总结", 3, []string{"用自己的话总结两个知识点及适用条件。", "布置一个同类变式问题，记录仍需澄清之处。"}},
		}, Resources: []string{"本主题原创示例讲义[1]", "白板、练习纸及本学科常用学习工具"}, CitationIndices: []int{1},
	}
}

type JuniorImportResult struct{ Created, Skipped int }

// ImportJunior is an explicit local maintenance action, never called during server startup.
// Materials and a resumable ledger commit first; private resources commit atomically per unit.
func (s *Store) ImportJunior(ctx context.Context, u auth.User, uploadDir string, ix *retrieval.Indexer) (JuniorImportResult, error) {
	out := JuniorImportResult{}
	units, e := JuniorUnits()
	if e != nil {
		return out, e
	}
	lock, e := s.DB.Conn(ctx)
	if e != nil {
		return out, e
	}
	defer lock.Close()
	key := fmt.Sprintf("junior-import-%d-%d", u.ClassID, u.ID)
	var held int
	if e = lock.QueryRowContext(ctx, "SELECT GET_LOCK(?,0)", key).Scan(&held); e != nil || held != 1 {
		return out, ErrBusy
	}
	defer lock.ExecContext(context.WithoutCancel(ctx), "SELECT RELEASE_LOCK(?)", key)
	for i, unit := range units {
		done, err := s.importJuniorUnit(ctx, u, unit, i, uploadDir, ix)
		if err != nil {
			return out, fmt.Errorf("主题%s未完成（可重试）: %w", unit.Key, err)
		}
		if done {
			out.Skipped++
		} else {
			out.Created++
		}
	}
	return out, nil
}
func (s *Store) importJuniorUnit(ctx context.Context, u auth.User, v JuniorUnit, offset int, dir string, ix *retrieval.Indexer) (bool, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return false, e
	}
	defer tx.Rollback()
	if e = validTeacher(ctx, tx, u); e != nil {
		return false, e
	}
	var material, entry int64
	var state string
	e = tx.QueryRowContext(ctx, `SELECT material_id,entry_id,status FROM curriculum_seed_items WHERE class_id=? AND created_by=? AND batch=? AND unit_key=? FOR UPDATE`, u.ClassID, u.ID, JuniorBatch, v.Key).Scan(&material, &entry, &state)
	if e == nil && state == "complete" {
		return true, nil
	}
	if e != nil && e != sql.ErrNoRows {
		return false, e
	}
	if e == sql.ErrNoRows {
		h := sha256.Sum256([]byte(fmt.Sprintf("%d/%d/%s/%s", u.ClassID, u.ID, JuniorBatch, v.Key)))
		file := filepath.Join(dir, "junior-"+hex.EncodeToString(h[:])+".md")
		if e = os.MkdirAll(dir, 0755); e != nil {
			return false, e
		}
		f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return false, fmt.Errorf("示例文件已存在或不可写，需检查批次备份: %w", err)
		}
		_, err = f.WriteString(v.lecture())
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			os.Remove(file)
			return false, fmt.Errorf("讲义写入失败")
		}
		committed := false
		defer func() {
			if !committed {
				os.Remove(file)
			}
		}()
		var r sql.Result
		r, e = tx.ExecContext(ctx, `INSERT INTO materials(class_id,title,file_path,uploaded_by) VALUES(?,?,?,?)`, u.ClassID, "【示例】"+subjects.Name(v.Subject)+" · "+v.chapter()+".md", file, u.ID)
		if e != nil {
			return false, e
		}
		material, e = r.LastInsertId()
		if e != nil {
			return false, e
		}
		r, e = tx.ExecContext(ctx, `INSERT INTO knowledge_entries(material_id,class_id,body_text) VALUES(?,?,?)`, material, u.ClassID, v.lecture())
		if e != nil {
			return false, e
		}
		entry, e = r.LastInsertId()
		if e != nil {
			return false, e
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO curriculum_seed_items(class_id,created_by,batch,unit_key,material_id,entry_id,resources_json) VALUES(?,?,?,?,?,?,JSON_OBJECT())`, u.ClassID, u.ID, JuniorBatch, v.Key, material, entry)
		if e != nil {
			return false, e
		}
		if e = tx.Commit(); e != nil {
			return false, e
		}
		committed = true
	} else {
		if e = tx.Commit(); e != nil {
			return false, e
		}
	}
	var currentBody string
	if e = s.DB.QueryRowContext(ctx, `SELECT body_text FROM knowledge_entries WHERE id=? AND material_id=? AND class_id=?`, entry, material, u.ClassID).Scan(&currentBody); e != nil {
		return false, e
	}
	if currentBody != v.lecture() {
		return false, ErrConflict
	}
	var ready, all int
	if e = s.DB.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(index_status='ready'),0) FROM knowledge_chunks WHERE material_id=? AND class_id=?`, material, u.ClassID).Scan(&all, &ready); e != nil {
		return false, e
	}
	if all == 0 || ready != all {
		if ix == nil {
			return false, ErrEvidence
		}
		if _, e = ix.IndexMaterial(ctx, material, entry, u.ClassID, "", "auto", chunking.Options{}); e != nil {
			return false, e
		}
		if e = s.DB.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(index_status='ready'),0) FROM knowledge_chunks WHERE material_id=? AND class_id=?`, material, u.ClassID).Scan(&all, &ready); e != nil {
			return false, e
		}
		if all == 0 || ready != all {
			return false, ErrEvidence
		}
	}
	tx, e = s.DB.BeginTx(ctx, nil)
	if e != nil {
		return false, e
	}
	defer tx.Rollback()
	if e = validTeacher(ctx, tx, u); e != nil {
		return false, e
	}
	var chunk int64
	if e = tx.QueryRowContext(ctx, `SELECT id FROM knowledge_chunks WHERE material_id=? AND class_id=? AND index_status='ready' ORDER BY chunk_index LIMIT 1 FOR SHARE`, material, u.ClassID).Scan(&chunk); e != nil {
		return false, e
	}
	ref, e := evidence(ctx, tx, u, chunk, material, true)
	if e != nil {
		return false, e
	}
	ref.Index = 1
	refs := []Evidence{ref}
	ids := []int64{}
	for _, name := range v.Points {
		in := PointInput{Subject: v.Subject, Name: name, Chapter: v.chapter(), Description: strings.Join(v.Facts, "；"), MaterialIDs: []int64{material}}
		if e = in.Validate(false); e != nil {
			return false, e
		}
		r, err := tx.ExecContext(ctx, `INSERT INTO knowledge_points(class_id,subject,name,chapter,description,created_by) VALUES(?,?,?,?,?,?)`, u.ClassID, v.Subject, name, v.chapter(), in.Description, u.ID)
		if err != nil {
			return false, normalize(err)
		}
		id, err := r.LastInsertId()
		if err != nil {
			return false, err
		}
		ids = append(ids, id)
		if _, e = tx.ExecContext(ctx, `INSERT INTO knowledge_point_materials(point_id,material_id) VALUES(?,?)`, id, material); e != nil {
			return false, e
		}
	}
	if e = graphScope(ctx, tx, u, v.Subject); e != nil {
		return false, e
	}
	edges, e := graphEdges(ctx, tx, u.ClassID, v.Subject)
	if e != nil {
		return false, e
	}
	edge, e := insertGraphEdge(ctx, tx, u, v.Subject, EdgeInput{FromID: ids[0], ToID: ids[1], Kind: "related"}, &edges)
	if e != nil {
		return false, e
	}
	in := BankInput{SourceInput: SourceInput{Subject: v.Subject, MaterialIDs: []int64{material}, ClientKey: JuniorBatch + "-" + v.Key}, KnowledgePointIDs: ids, QuestionTypes: []string{"single_choice", "multiple_choice", "true_false", "short_answer"}, Difficulty: "medium", QuestionCount: 4}
	items := v.questions(ids, offset)
	if e = in.Validate(); e != nil {
		return false, e
	}
	if e = validateQuestions(items, in, 1, true); e != nil {
		return false, e
	}
	if e = checkBankDuplicates(ctx, tx, u, v.Subject, items, 0); e != nil {
		return false, e
	}
	marshal := func(v any) string { b, _ := json.Marshal(v); return string(b) }
	r, e := tx.ExecContext(ctx, `INSERT INTO question_drafts(class_id,created_by,subject,input_json,questions_json,citations_json,bank_ids_json,client_key,input_hash,status,version) VALUES(?,?,?,?,?,?,JSON_ARRAY(),?,?,'approved',2)`, u.ClassID, u.ID, v.Subject, marshal(in), marshal(items), marshal(refs), in.ClientKey, in.Hash())
	if e != nil {
		return false, e
	}
	draft, e := r.LastInsertId()
	if e != nil {
		return false, e
	}
	banks := []int64{}
	for _, q := range items {
		r, e = tx.ExecContext(ctx, `INSERT INTO bank_questions(class_id,created_by,subject,origin_draft_id,question_json,citations_json,stem,stem_hash) VALUES(?,?,?,?,?,?,?,?)`, u.ClassID, u.ID, v.Subject, draft, marshal(q), marshal(refs), q.Stem, q.StemHash())
		if e != nil {
			return false, e
		}
		id, err := r.LastInsertId()
		if err != nil {
			return false, err
		}
		banks = append(banks, id)
	}
	if _, e = tx.ExecContext(ctx, `UPDATE question_drafts SET bank_ids_json=? WHERE id=?`, marshal(banks), draft); e != nil {
		return false, e
	}
	lesson := v.lesson()
	if e = lesson.Validate(45, 1); e != nil {
		return false, e
	}
	li := LessonInput{Subject: v.Subject, Chapter: "【示例】" + v.chapter(), Objectives: strings.Join(lesson.Goals, "；"), Periods: 1, MinutesPerPeriod: 45, MaterialIDs: []int64{material}, ClientKey: in.ClientKey}
	if e = li.Validate(); e != nil {
		return false, e
	}
	r, e = tx.ExecContext(ctx, `INSERT INTO lesson_plans(class_id,created_by,subject,title,input_json,body,citations_json,client_key,input_hash) VALUES(?,?,?,?,?,?,?,?,?)`, u.ClassID, u.ID, v.Subject, li.Chapter+" · 教案", marshal(li), lesson.Body(), marshal(refs), li.ClientKey, li.Hash())
	if e != nil {
		return false, e
	}
	lessonID, e := r.LastInsertId()
	if e != nil {
		return false, e
	}
	resources := map[string]any{"point_ids": ids, "edge_id": edge.ID, "draft_id": draft, "bank_ids": banks, "lesson_id": lessonID, "origin": "curated-example"}
	if _, e = tx.ExecContext(ctx, `UPDATE curriculum_seed_items SET status='complete',resources_json=? WHERE class_id=? AND created_by=? AND batch=? AND unit_key=?`, marshal(resources), u.ClassID, u.ID, JuniorBatch, v.Key); e != nil {
		return false, e
	}
	return false, tx.Commit()
}
