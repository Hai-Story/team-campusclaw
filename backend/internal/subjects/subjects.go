package subjects

import "strings"

type Subject struct {
	ID, Name string
	Words    []string
}

var All = []Subject{
	{"chinese", "语文", []string{"语文", "阅读", "写作", "作文", "古诗", "春晓", "chinese"}},
	{"math", "数学", []string{"数学", "方程", "分数", "几何", "math"}},
	{"english", "英语", []string{"英语", "english"}},
	{"physics", "物理", []string{"物理", "磁铁", "运动学", "physics"}},
	{"chemistry", "化学", []string{"化学", "酸碱", "chemistry"}},
	{"biology", "生物", []string{"生物", "光合", "biology"}},
	{"politics", "政治", []string{"政治", "思想品德", "品德", "道德", "法治", "思政", "morality", "ethics", "politics"}},
	{"history", "历史", []string{"历史", "history"}},
	{"geography", "地理", []string{"地理", "geography"}},
}

func Name(id string) string {
	for _, s := range All {
		if s.ID == id {
			return s.Name
		}
	}
	return ""
}
func Classify(title string) string {
	title = strings.ToLower(title)
	for _, s := range All {
		for _, w := range s.Words {
			if strings.Contains(title, w) {
				return s.ID
			}
		}
	}
	if strings.Contains(title, "科学") || strings.Contains(title, "science") {
		return "physics"
	}
	return "other"
}
