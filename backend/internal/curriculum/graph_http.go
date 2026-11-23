package curriculum

import "net/http"

// @api {"method":"GET","path":"/api/knowledge-graph","summary":"读取本学科正式图谱和终评薄弱样本","auth":"session","status":200,"response":{"$ref":"#/components/schemas/KnowledgeGraph"},"parameters":[{"name":"subject","in":"query","schema":{"type":"string","enum":["chinese","math","english","physics","chemistry","biology","politics","history","geography"],"default":"math"}}],"errors":{"400":"学科非法","401":"需要登录","409":"图谱超过上限","503":"服务暂不可用"}}
// @api {"method":"POST","path":"/api/knowledge-graph/edges","summary":"教师新增或幂等读取正式关系","auth":"teacher","status":201,"request":{"$ref":"#/components/schemas/GraphEdgeInput"},"response":{"$ref":"#/components/schemas/GraphEdge"},"errors":{"400":"输入非法","401":"需要登录","403":"仅教师","404":"节点不存在","409":"成环或超过上限","413":"请求过大","415":"类型非法","503":"服务暂不可用"}}
// @api {"method":"DELETE","path":"/api/knowledge-graph/edges/{id}","summary":"教师按版本删除本班关系","auth":"teacher","status":204,"request":{"$ref":"#/components/schemas/ExpectedVersion"},"errors":{"400":"输入非法","401":"需要登录","403":"仅教师","404":"资源不存在","409":"版本冲突","413":"请求过大","415":"类型非法","503":"服务暂不可用"}}
// @api {"method":"GET","path":"/api/knowledge-graph/nodes/{id}/resources","summary":"读取本班节点资源、即时错题和终评样本","auth":"session","status":200,"response":{"type":"object","properties":{"point":{"$ref":"#/components/schemas/KnowledgePoint"},"materials":{"type":"array","items":{"type":"object"}},"assignments":{"type":"array","items":{"type":"object"}},"recommended_points":{"type":"array","items":{"type":"object"}},"review_materials":{"type":"array","items":{"type":"object"}},"mistakes":{"type":"array","items":{"type":"object"}},"reviewed":{"type":"integer"},"incorrect":{"type":"integer"},"weak":{"type":"boolean"}}},"errors":{"400":"标识非法","401":"需要登录","404":"资源不存在","503":"服务暂不可用"}}
// @api {"method":"POST","path":"/api/knowledge-graph/drafts/generate","summary":"从指定讲义生成教师私有图谱候选","auth":"teacher","status":200,"request":{"$ref":"#/components/schemas/SourceInput"},"response":{"type":"object","properties":{"status":{"type":"string","enum":["generated","insufficient_evidence"]},"message":{"type":"string"},"draft":{"allOf":[{"$ref":"#/components/schemas/GraphDraft"}],"nullable":true}}},"errors":{"400":"输入非法","401":"需要登录","403":"仅教师","404":"资料不存在","409":"键冲突或来源变化","413":"请求过大","415":"类型非法","429":"正在生成","503":"生成暂不可用"}}
// @api {"method":"GET","path":"/api/knowledge-graph/drafts","summary":"分页读取本人图谱候选","auth":"teacher","status":200,"response":{"type":"object","properties":{"items":{"type":"array","items":{"$ref":"#/components/schemas/GraphDraft"}},"page":{"type":"integer"},"page_size":{"type":"integer"}}},"errors":{"400":"筛选非法","401":"需要登录","403":"仅教师","503":"服务暂不可用"}}
// @api {"method":"GET","path":"/api/knowledge-graph/drafts/{id}","summary":"读取本人图谱候选与出处","auth":"teacher","status":200,"response":{"$ref":"#/components/schemas/GraphDraft"},"errors":{"400":"标识非法","401":"需要登录","403":"仅教师","404":"资源不存在","503":"服务暂不可用"}}
// @api {"method":"PUT","path":"/api/knowledge-graph/drafts/{id}","summary":"按版本修改待审节点及关系","auth":"teacher","status":200,"request":{"$ref":"#/components/schemas/GraphDraftUpdate"},"response":{"$ref":"#/components/schemas/GraphDraft"},"errors":{"400":"输入非法","401":"需要登录","403":"仅教师","404":"资源不存在","409":"状态或版本冲突","413":"请求过大","415":"类型非法","503":"服务暂不可用"}}
// @api {"method":"POST","path":"/api/knowledge-graph/drafts/{id}/approve","summary":"教师原子确认候选节点、关系及来源","auth":"teacher","status":200,"request":{"$ref":"#/components/schemas/ExpectedVersion"},"response":{"$ref":"#/components/schemas/GraphDraft"},"errors":{"400":"输入非法","401":"需要登录","403":"仅教师","404":"资源不存在","409":"版本、来源、节点或成环冲突","413":"请求过大","415":"类型非法","503":"服务暂不可用"}}
func mountGraph(mux *http.ServeMux, s *Service, protect func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /api/knowledge-graph", protect(s.GraphHTTP))
	mux.HandleFunc("POST /api/knowledge-graph/edges", protect(s.AddEdgeHTTP))
	mux.HandleFunc("DELETE /api/knowledge-graph/edges/{id}", protect(s.DeleteEdgeHTTP))
	mux.HandleFunc("GET /api/knowledge-graph/nodes/{id}/resources", protect(s.NodeResourcesHTTP))
	mux.HandleFunc("POST /api/knowledge-graph/drafts/generate", protect(s.GenerateGraphHTTP))
	mux.HandleFunc("GET /api/knowledge-graph/drafts", protect(s.ListGraphDraftsHTTP))
	mux.HandleFunc("GET /api/knowledge-graph/drafts/{id}", protect(s.GetGraphDraftHTTP))
	mux.HandleFunc("PUT /api/knowledge-graph/drafts/{id}", protect(s.UpdateGraphDraftHTTP))
	mux.HandleFunc("POST /api/knowledge-graph/drafts/{id}/approve", protect(s.ApproveGraphDraftHTTP))
}
func (s *Service) GraphHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "")
	if !ok {
		return
	}
	subject := "math"
	for k, v := range r.URL.Query() {
		if k != "subject" || len(v) != 1 {
			failure(w, ErrInvalid)
			return
		}
		subject = v[0]
	}
	if !ValidSubject(subject) {
		failure(w, ErrInvalid)
		return
	}
	v, e := s.Store.Graph(r.Context(), u, subject)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Service) AddEdgeHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	var in EdgeInput
	if !decode(w, r, &in) {
		return
	}
	if e := in.Validate(); e != nil {
		failure(w, e)
		return
	}
	v, e := s.Store.AddEdge(r.Context(), u, in)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 201, v)
}

type ExpectedVersion struct {
	ExpectedVersion int `json:"expected_version"`
}

func (s *Service) DeleteEdgeHTTP(w http.ResponseWriter, r *http.Request) {
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
	if e := s.Store.DeleteEdge(r.Context(), u, id, in.ExpectedVersion); e != nil {
		failure(w, e)
		return
	}
	write(w, 204, nil)
}
func (s *Service) NodeResourcesHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	v, e := s.Store.NodeResources(r.Context(), u, id)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Service) GenerateGraphHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	var in SourceInput
	if !decode(w, r, &in) {
		return
	}
	if e := in.Validate(); e != nil {
		failure(w, e)
		return
	}
	v, e := s.GenerateGraph(r.Context(), u, in)
	if r.Context().Err() != nil {
		return
	}
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Service) ListGraphDraftsHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	f, e := pointFilter(r)
	if e != nil || r.URL.Query().Has("favorites") {
		failure(w, ErrInvalid)
		return
	}
	v, e := s.Store.GraphDrafts(r.Context(), u, f)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, map[string]any{"items": v, "page": f.Page, "page_size": f.Size})
}
func (s *Service) GetGraphDraftHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	v, e := s.Store.GraphDraft(r.Context(), u, id)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Service) UpdateGraphDraftHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "teacher")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in GraphUpdate
	if !decode(w, r, &in) {
		return
	}
	if in.ExpectedVersion < 1 {
		failure(w, ErrInvalid)
		return
	}
	v, e := s.Store.UpdateGraphDraft(r.Context(), u, id, in)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Service) ApproveGraphDraftHTTP(w http.ResponseWriter, r *http.Request) {
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
	v, e := s.Store.ApproveGraphDraft(r.Context(), u, id, in.ExpectedVersion)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}
