package curriculum

import (
	"net/http"
	"strconv"
)

// @api {"method":"GET","path":"/api/homework-notifications","summary":"读取本人本班作业提醒与未读数","auth":"student","status":200,"parameters":[{"name":"page","in":"query","schema":{"type":"integer","minimum":1,"default":1}},{"name":"page_size","in":"query","schema":{"type":"integer","minimum":1,"maximum":100,"default":20}}],"response":{"type":"object","required":["items","page","page_size","unread_count"],"properties":{"items":{"type":"array","items":{"type":"object","required":["id","kind","resource_id","subject","title","created_at","read_at"],"properties":{"id":{"type":"integer"},"kind":{"type":"string","enum":["assignment","paper"]},"resource_id":{"type":"integer"},"subject":{"type":"string"},"title":{"type":"string"},"created_at":{"type":"string","format":"date-time"},"read_at":{"type":"string","format":"date-time","nullable":true}}}},"page":{"type":"integer"},"page_size":{"type":"integer"},"unread_count":{"type":"integer"}}},"errors":{"400":"分页参数非法","401":"需要登录","403":"仅学生可读取","503":"服务暂时不可用"}}
func (s *Service) ListHomeworkNotifications(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "student")
	if !ok {
		return
	}
	page, size := 1, 20
	for key, values := range r.URL.Query() {
		if (key != "page" && key != "page_size") || len(values) != 1 {
			failure(w, ErrInvalid)
			return
		}
		n, e := strconv.Atoi(values[0])
		if e != nil || n < 1 || n > 2147483647 {
			failure(w, ErrInvalid)
			return
		}
		if key == "page" {
			page = n
		} else {
			size = n
		}
	}
	if size > 100 {
		size = 100
	}
	v, e := s.Store.HomeworkNotifications(r.Context(), u, page, size)
	if e != nil {
		failure(w, e)
		return
	}
	write(w, 200, v)
}

// @api {"method":"PUT","path":"/api/homework-notifications/{id}/read","summary":"幂等标记本人本班提醒已读","auth":"student","status":204,"errors":{"400":"标识非法","401":"需要登录","403":"仅学生可修改","404":"资源不存在","503":"服务暂时不可用"}}
func (s *Service) ReadHomeworkNotification(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(w, r, "student")
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if e := s.Store.ReadHomeworkNotification(r.Context(), u, id); e != nil {
		failure(w, e)
		return
	}
	write(w, 204, nil)
}
