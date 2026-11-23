package curriculum

import "testing"

func TestJuniorCatalogQuality(t *testing.T) {
	units, e := JuniorUnits()
	if e != nil {
		t.Fatal(e)
	}
	grades := map[int]bool{}
	stems := map[string]bool{}
	answers := map[string]bool{}
	for i, u := range units {
		grades[u.Grade] = true
		if len([]rune(u.lecture())) > 800 {
			t.Fatalf("lecture needs multiple-citation handling: %s", u.Key)
		}
		in := BankInput{SourceInput: SourceInput{Subject: u.Subject, MaterialIDs: []int64{1}, ClientKey: JuniorBatch + "-" + u.Key}, KnowledgePointIDs: []int64{1, 2}, QuestionTypes: []string{"single_choice", "multiple_choice", "true_false", "short_answer"}, Difficulty: "medium", QuestionCount: 4}
		q := u.questions(in.KnowledgePointIDs, i)
		if e = in.Validate(); e != nil {
			t.Fatal(u.Key, e)
		}
		if e = validateQuestions(q, in, 1, true); e != nil {
			t.Fatal(u.Key, e)
		}
		for _, item := range q {
			if stems[item.StemHash()] {
				t.Fatal("duplicate", u.Key)
			}
			stems[item.StemHash()] = true
		}
		answers[q[0].Answer[0]] = true
		if e = u.lesson().Validate(45, 1); e != nil {
			t.Fatal(u.Key, e)
		}
	}
	if len(units) != 54 || len(stems) != 216 || len(grades) != 3 || len(answers) != 4 {
		t.Fatal(len(units), len(stems), grades, answers)
	}
}
