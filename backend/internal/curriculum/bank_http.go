package curriculum

import "net/http"

// @api {"method":"POST","path":"/api/question-drafts/generate","summary":"按知识点及本班资料生成私有题目草稿","auth":"teacher","status":200,"request":{"$ref":"#/components/schemas/BankInput"},"response":{"type":"object","properties":{"status":{"type":"string","enum":["generated","insufficient_evidence"]},"message":{"type":"string"},"draft":{"allOf":[{"$ref":"#/components/schemas/QuestionDraft"}],"nullable":true}}},"errors":{"400":"输入非法","401":"需要登录","403":"仅教师","404":"知识点或资料不存在","409":"请求键、重复题或来源冲突","413":"请求过大","415":"类型非法","429":"正在生成","503":"生成暂不可用"}}
// @api {"method":"GET","path":"/api/question-drafts","summary":"分页读取本人命题草稿","auth":"teacher","status":200,"response":{"type":"object","properties":{"items":{"type":"array","items":{"$ref":"#/components/schemas/QuestionDraft"}},"page":{"type":"integer"},"page_size":{"type":"integer"}}},"errors":{"400":"筛选非法","401":"需要登录","403":"仅教师","503":"服务暂不可用"}}
// @api {"method":"GET","path":"/api/question-drafts/{id}","summary":"读取本人题目草稿及真实出处","auth":"teacher","status":200,"response":{"$ref":"#/components/schemas/QuestionDraft"},"errors":{"400":"标识非法","401":"需要登录","403":"仅教师","404":"资源不存在","503":"服务暂不可用"}}
// @api {"method":"PUT","path":"/api/question-drafts/{id}","summary":"按版本保存待审题目","auth":"teacher","status":200,"request":{"type":"object","additionalProperties":false,"required":["questions","expected_version"],"properties":{"questions":{"type":"array","minItems":1,"maxItems":10,"items":{"$ref":"#/components/schemas/QuestionItem"}},"expected_version":{"type":"integer","minimum":1}}},"response":{"$ref":"#/components/schemas/QuestionDraft"},"errors":{"400":"逐题输入非法","401":"需要登录","403":"仅教师","404":"资源不存在","409":"版本或重复题冲突","413":"请求过大","415":"类型非法","503":"服务暂不可用"}}
// @api {"method":"POST","path":"/api/question-drafts/{id}/approve","summary":"原子审核命题草稿并入私有题库","auth":"teacher","status":200,"request":{"$ref":"#/components/schemas/ExpectedVersion"},"response":{"$ref":"#/components/schemas/QuestionDraft"},"errors":{"400":"输入非法","401":"需要登录","403":"仅教师","404":"资源不存在","409":"版本、重复题或来源冲突","413":"请求过大","415":"类型非法","503":"服务暂不可用"}}
// @api {"method":"GET","path":"/api/bank-questions","summary":"分页查询本人私有题库","auth":"teacher","status":200,"response":{"type":"object","properties":{"items":{"type":"array","items":{"$ref":"#/components/schemas/BankQuestion"}},"page":{"type":"integer"},"page_size":{"type":"integer"}}},"errors":{"400":"筛选非法","401":"需要登录","403":"仅教师","503":"服务暂不可用"}}
// @api {"method":"GET","path":"/api/bank-questions/{id}","summary":"读取本人题库题目、答案和解析","auth":"teacher","status":200,"response":{"$ref":"#/components/schemas/BankQuestion"},"errors":{"400":"标识非法","401":"需要登录","403":"仅教师","404":"资源不存在","503":"服务暂不可用"}}
// @api {"method":"PUT","path":"/api/bank-questions/{id}","summary":"教师确认复核后按版本修改题库题目","auth":"teacher","status":200,"request":{"type":"object","additionalProperties":false,"required":["question","expected_version","confirm_review"],"properties":{"question":{"$ref":"#/components/schemas/QuestionItem"},"expected_version":{"type":"integer","minimum":1},"confirm_review":{"type":"boolean","enum":[true]}}},"response":{"$ref":"#/components/schemas/BankQuestion"},"errors":{"400":"未确认复核或题目非法","401":"需要登录","403":"仅教师","404":"资源不存在","409":"版本或重复题冲突","413":"请求过大","415":"类型非法","503":"服务暂不可用"}}
// @api {"method":"DELETE","path":"/api/bank-questions/{id}","summary":"按版本删除本人题库条目并保留试卷快照","auth":"teacher","status":204,"request":{"$ref":"#/components/schemas/ExpectedVersion"},"errors":{"400":"输入非法","401":"需要登录","403":"仅教师","404":"资源不存在","409":"版本冲突","413":"请求过大","415":"类型非法","503":"服务暂不可用"}}
// @api {"method":"POST","path":"/api/paper-drafts","summary":"保存有序选题及配分的私有试卷快照","auth":"teacher","status":201,"request":{"$ref":"#/components/schemas/PaperInput"},"response":{"$ref":"#/components/schemas/PaperDraft"},"errors":{"400":"标题、重复选题或分值非法","401":"需要登录","403":"仅教师","404":"题目不存在","409":"键或题库版本冲突","413":"请求过大","415":"类型非法","503":"服务暂不可用"}}
// @api {"method":"GET","path":"/api/paper-drafts","summary":"分页读取本人试卷草稿摘要","auth":"teacher","status":200,"response":{"type":"object","properties":{"items":{"type":"array","items":{"$ref":"#/components/schemas/PaperSummary"}},"page":{"type":"integer"},"page_size":{"type":"integer"}}},"errors":{"400":"筛选非法","401":"需要登录","403":"仅教师","503":"服务暂不可用"}}
// @api {"method":"GET","path":"/api/paper-drafts/{id}","summary":"恢复本人试卷的题面、顺序与分值快照","auth":"teacher","status":200,"response":{"$ref":"#/components/schemas/PaperDraft"},"errors":{"400":"标识非法","401":"需要登录","403":"仅教师","404":"资源不存在","503":"服务暂不可用"}}
// @api {"method":"PUT","path":"/api/paper-drafts/{id}","summary":"按版本更新试卷排序配分及明确选择的题面","auth":"teacher","status":200,"request":{"$ref":"#/components/schemas/PaperUpdate"},"response":{"$ref":"#/components/schemas/PaperDraft"},"errors":{"400":"输入非法","401":"需要登录","403":"仅教师","404":"题目不存在","409":"试卷或题库版本冲突","413":"请求过大","415":"类型非法","503":"服务暂不可用"}}
func mountBank(mux *http.ServeMux, s *Service, protect func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("POST /api/question-drafts/generate", protect(s.GenerateQuestionsHTTP))
	mux.HandleFunc("GET /api/question-drafts", protect(s.ListQuestionDraftsHTTP))
	mux.HandleFunc("GET /api/question-drafts/{id}", protect(s.GetQuestionDraftHTTP))
	mux.HandleFunc("PUT /api/question-drafts/{id}", protect(s.UpdateQuestionDraftHTTP))
	mux.HandleFunc("POST /api/question-drafts/{id}/approve", protect(s.ApproveQuestionDraftHTTP))
	mux.HandleFunc("GET /api/bank-questions", protect(s.ListBankQuestionsHTTP))
	mux.HandleFunc("GET /api/bank-questions/{id}", protect(s.GetBankQuestionHTTP))
	mux.HandleFunc("PUT /api/bank-questions/{id}", protect(s.UpdateBankQuestionHTTP))
	mux.HandleFunc("DELETE /api/bank-questions/{id}", protect(s.DeleteBankQuestionHTTP))
	mux.HandleFunc("POST /api/paper-drafts", protect(s.CreatePaperHTTP))
	mux.HandleFunc("GET /api/paper-drafts", protect(s.ListPapersHTTP))
	mux.HandleFunc("GET /api/paper-drafts/{id}", protect(s.GetPaperHTTP))
	mux.HandleFunc("PUT /api/paper-drafts/{id}", protect(s.UpdatePaperHTTP))
}
func (s *Service) GenerateQuestionsHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	var in BankInput
	if !decode(w, r, &in) {
		return
	}
	if e := in.Validate(); e != nil {
		failure(w, e)
		return
	}
	v, e := s.GenerateQuestions(r.Context(), u, in)
	if r.Context().Err() != nil {
		return
	}
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Service) ListQuestionDraftsHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	f, e := pointFilter(r)
	if e != nil || r.URL.Query().Has("favorites") {
		failure(w, ErrInvalid)
		return
	}
	v, e := s.Store.QuestionDrafts(r.Context(), u, f)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, map[string]any{"items": v, "page": f.Page, "page_size": f.Size})
}
func (s *Service) GetQuestionDraftHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	v, e := s.Store.QuestionDraft(r.Context(), u, id)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Service) UpdateQuestionDraftHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in DraftQuestionsUpdate
	if !decode(w, r, &in) {
		return
	}
	if in.ExpectedVersion < 1 {
		failure(w, ErrInvalid)
		return
	}
	v, e := s.Store.UpdateQuestionDraft(r.Context(), u, id, in)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Service) ApproveQuestionDraftHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in ExpectedVersion
	if !decode(w, r, &in) {
		return
	}
	if in.ExpectedVersion < 1 {
		failure(w, ErrInvalid)
		return
	}
	v, e := s.Store.ApproveQuestionDraft(r.Context(), u, id, in.ExpectedVersion)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Service) ListBankQuestionsHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	f, e := pointFilter(r)
	if e != nil || r.URL.Query().Has("favorites") {
		failure(w, ErrInvalid)
		return
	}
	v, e := s.Store.BankQuestions(r.Context(), u, f)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, map[string]any{"items": v, "page": f.Page, "page_size": f.Size})
}
func (s *Service) GetBankQuestionHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	v, e := s.Store.BankQuestion(r.Context(), u, id)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Service) UpdateBankQuestionHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in BankQuestionUpdate
	if !decode(w, r, &in) {
		return
	}
	if in.ExpectedVersion < 1 || !in.ConfirmReview {
		failure(w, ErrInvalid)
		return
	}
	v, e := s.Store.UpdateBankQuestion(r.Context(), u, id, in)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Service) DeleteBankQuestionHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in ExpectedVersion
	if !decode(w, r, &in) {
		return
	}
	if in.ExpectedVersion < 1 {
		failure(w, ErrInvalid)
		return
	}
	if e := s.Store.DeleteBankQuestion(r.Context(), u, id, in.ExpectedVersion); e != nil {
		failure(w, e)
		return
	}
	write(w, 204, nil)
}
func (s *Service) CreatePaperHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	var in PaperInput
	if !decode(w, r, &in) {
		return
	}
	if e := in.Validate(); e != nil {
		failure(w, e)
		return
	}
	v, e := s.Store.CreatePaper(r.Context(), u, in)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 201, v)
}
func (s *Service) ListPapersHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	f, e := pointFilter(r)
	if e != nil || r.URL.Query().Has("favorites") {
		failure(w, ErrInvalid)
		return
	}
	v, e := s.Store.Papers(r.Context(), u, f)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, map[string]any{"items": v, "page": f.Page, "page_size": f.Size})
}
func (s *Service) GetPaperHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	v, e := s.Store.Paper(r.Context(), u, id)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Service) UpdatePaperHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in PaperUpdate
	if !decode(w, r, &in) {
		return
	}
	if e := in.Validate(); e != nil {
		failure(w, e)
		return
	}
	v, e := s.Store.UpdatePaper(r.Context(), u, id, in)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}
