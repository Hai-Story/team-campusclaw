package curriculum

import (
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"time"
)

var ErrDiagnosisSize = errors.New("正式样本超过10000条，请缩小日期范围")

func diagnosisFilter(r *http.Request) (DiagnosisFilter, error) {
	f := DiagnosisFilter{Subject: "math", Days: 30, Page: 1, Size: 20}
	for k, vs := range r.URL.Query() {
		if len(vs) != 1 {
			return f, ErrInvalid
		}
		v := vs[0]
		if k == "subject" {
			f.Subject = v
			continue
		}
		n, e := strconv.ParseInt(v, 10, 32)
		if e != nil || n < 1 {
			return f, ErrInvalid
		}
		switch k {
		case "days":
			f.Days = int(n)
		case "page":
			f.Page = int(n)
		case "page_size":
			f.Size = int(n)
		case "student_id":
			f.StudentID = n
		default:
			return f, ErrInvalid
		}
	}
	return f, f.Validate(time.Now())
}

// @api {"method":"GET","path":"/api/learning-diagnosis","summary":"按教师终评查询本人或本班学情诊断","auth":"login","status":200,"response":{"$ref":"#/components/schemas/DiagnosisReport"},"parameters":[{"name":"subject","in":"query","schema":{"type":"string","enum":["chinese","math","english","physics","chemistry","biology","politics","history","geography"],"default":"math"}},{"name":"days","in":"query","schema":{"type":"integer","minimum":1,"maximum":180,"default":30}},{"name":"student_id","in":"query","schema":{"type":"integer","minimum":1},"description":"仅教师可选本班学生"},{"name":"page","in":"query","schema":{"type":"integer","minimum":1,"default":1}},{"name":"page_size","in":"query","schema":{"type":"integer","minimum":1,"maximum":100,"default":20}}],"errors":{"400":"范围非法或样本过多","401":"需要登录","404":"学生不存在或无权","503":"服务暂时不可用"}}
func (s *Service) Diagnosis(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "")
	if !ok {
		return
	}
	f, e := diagnosisFilter(r)
	if e != nil {
		failure(w, e)
		return
	}
	v, e := s.Store.Diagnosis(r.Context(), u, f)
	if errors.Is(e, ErrDiagnosisSize) {
		write(w, 400, map[string]string{"error": e.Error()})
		return
	}
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}

// @api {"method":"GET","path":"/api/learning-diagnosis/samples/{id}","summary":"回查授权的当前终评作答","auth":"login","status":200,"response":{"$ref":"#/components/schemas/DiagnosisSample"},"errors":{"400":"标识非法","401":"需要登录","404":"作答不存在或无权","503":"服务暂时不可用"}}
func (s *Service) DiagnosisSample(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	v, e := s.Store.DiagnosisSample(r.Context(), u, id)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}
func mountDiagnosis(mux *http.ServeMux, s *Service, protect func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("POST /api/learning-diagnosis/explanation", protect(s.DiagnosisExplanation))
	mux.HandleFunc("GET /api/learning-diagnosis", protect(s.Diagnosis))
	mux.HandleFunc("GET /api/learning-diagnosis/samples/{id}", protect(s.DiagnosisSample))
}

// @api {"method":"POST","path":"/api/learning-diagnosis/explanation","summary":"解释已确认的教师终评汇总","auth":"login","status":200,"request":{"type":"object","additionalProperties":false,"required":["subject","days","fingerprint"],"properties":{"subject":{"type":"string","enum":["chinese","math","english","physics","chemistry","biology","politics","history","geography"]},"days":{"type":"integer","minimum":1,"maximum":180},"student_id":{"type":"integer","minimum":1},"fingerprint":{"type":"string","pattern":"^[a-f0-9]{64}$"}}},"response":{"type":"object","required":["status","explanation","fingerprint"],"properties":{"status":{"type":"string","enum":["generated","insufficient_data"]},"explanation":{"type":"string","maxLength":2000},"fingerprint":{"type":"string"}}},"errors":{"400":"范围或指纹非法","401":"需要登录","404":"学生不存在或无权","409":"统计已更新，请刷新","413":"请求过大","415":"类型错误","429":"正在生成，请稍后重试","503":"解释暂时不可用"}}
func (s *Service) DiagnosisExplanation(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "")
	if !ok {
		return
	}
	var in struct {
		Subject     string `json:"subject"`
		Days        int    `json:"days"`
		StudentID   int64  `json:"student_id"`
		Fingerprint string `json:"fingerprint"`
	}
	if !decode(w, r, &in) {
		return
	}
	f := DiagnosisFilter{Subject: in.Subject, Days: in.Days, StudentID: in.StudentID, Page: 1, Size: 20}
	decoded, e := hex.DecodeString(in.Fingerprint)
	if len(in.Fingerprint) != 64 || e != nil || hex.EncodeToString(decoded) != in.Fingerprint || f.Validate(time.Now()) != nil {
		failure(w, ErrInvalid)
		return
	}
	v, e := s.ExplainDiagnosis(r.Context(), u, f, in.Fingerprint)
	if errors.Is(e, ErrConflict) {
		write(w, 409, map[string]string{"error": "统计已更新，请刷新后重新分析"})
		return
	}
	if errors.Is(e, ErrDiagnosisSize) {
		write(w, 400, map[string]string{"error": e.Error()})
		return
	}
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}
