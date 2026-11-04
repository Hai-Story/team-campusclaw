package tutor

import (
 "bufio"
 "bytes"
 "context"
 "database/sql"
 "encoding/json"
 "fmt"
 "io"
 "net/http"
 "net/http/httptest"
 "os"
 "strings"
 "testing"
 "time"
 "campusclaw/backend/internal/auth"
 "campusclaw/backend/internal/config"
 "campusclaw/backend/internal/db"
 "campusclaw/backend/internal/retrieval"
)
func integrationDB(t *testing.T)(*sql.DB,*config.Config){t.Helper();if os.Getenv("TUTOR_INTEGRATION")!="1"{t.Skip("requires existing compose database and TUTOR_INTEGRATION=1")};cfg,e:=config.Load();if e!=nil{t.Fatal(e)};d,e:=db.Open(cfg);if e!=nil{t.Fatal(e)};if e=d.Ping();e!=nil{t.Fatal(e)};t.Cleanup(func(){d.Close()});return d,cfg}
func TestBoundRetrievalIntegration(t *testing.T){
 d,cfg:=integrationDB(t);ids:=[]int64{};chunks:=[]int64{}
 for _,class:=range []int{1,1,2}{res,e:=d.Exec(`INSERT INTO materials(class_id,title,file_path,uploaded_by) VALUES(?,'检索验收材料','',1)`,class);if e!=nil{t.Fatal(e)};id,_:=res.LastInsertId();ids=append(ids,id)
  res,e=d.Exec(`INSERT INTO knowledge_chunks(knowledge_entry_id,material_id,class_id,chunk_index,chunk_text,char_start,char_end,index_status) VALUES(1,?,?,0,'共同词 方程去括号',0,10,'ready')`,id,class);if e!=nil{t.Fatal(e)};cid,_:=res.LastInsertId();chunks=append(chunks,cid)
 }
 t.Cleanup(func(){for _,id:=range ids{d.Exec(`DELETE FROM knowledge_chunks WHERE material_id=?`,id);d.Exec(`DELETE FROM materials WHERE id=?`,id)}})
 mock:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  if strings.HasSuffix(r.URL.Path,"/embeddings"){fmt.Fprint(w,`{"data":[{"embedding":[1]}]}`);return};if r.Method=="GET"{fmt.Fprint(w,`{"result":{}}`);return}
  var v map[string]any;json.NewDecoder(r.Body).Decode(&v);must:=v["filter"].(map[string]any)["must"].([]any);if len(must)!=2{t.Error("missing class/material restriction")}
  // Deliberately return unbound and cross-class points; MySQL must exclude both.
  fmt.Fprintf(w,`{"result":[{"id":%d,"score":0.9},{"id":%d,"score":0.8},{"id":%d,"score":0.7}]}`,chunks[1],chunks[2],chunks[0])
 }));defer mock.Close();cfg.AskTopK=4
 s:=retrieval.Searcher{DB:d,Cfg:cfg,Embed:&retrieval.EmbeddingClient{BaseURL:mock.URL,Dim:1},Qdr:&retrieval.QdrantClient{BaseURL:mock.URL}}
 hits,e:=s.SearchBound(context.Background(),1,"共同词",ids[:1],4);if e!=nil||len(hits)!=1||hits[0].MaterialID!=ids[0]{t.Fatal(hits,e)}
 empty,e:=s.SearchBound(context.Background(),1,"共同词",[]int64{},4);if e!=nil||len(empty)!=0{t.Fatal(empty,e)}
}
func TestParallelFailureAndCancellationIntegration(t *testing.T){
 d,cfg:=integrationDB(t);gate:=make(chan struct{});defer func(){select{case <-gate:default:close(gate)}}()
 mock:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  var in struct{Stream bool `json:"stream"`;Messages []retrieval.ChatMessage `json:"messages"`};json.NewDecoder(r.Body).Decode(&in);if !in.Stream{t.Error("expected stream=true")};q:=in.Messages[len(in.Messages)-1].Content
  w.Header().Set("Content-Type","text/event-stream")
  fmt.Fprint(w,"data: {\"choices\":[{\"delta\":{\"content\":\"部分[\"}}]}\n\n");w.(http.Flusher).Flush()
  fmt.Fprint(w,"data: {\"choices\":[{\"delta\":{\"content\":\"9]正文\"}}]}\n\n");w.(http.Flusher).Flush()
  if strings.Contains(q,"failure"){return}
  if strings.Contains(q,"cancel"){<-r.Context().Done();return}
  select{case <-gate:case <-r.Context().Done():return};fmt.Fprint(w,"data: [DONE]\n\n")
 }));defer mock.Close()
 s:=NewService(d,&retrieval.Searcher{DB:d,Cfg:cfg},&retrieval.ChatClient{BaseURL:mock.URL});a:=auth.NewService(d,cfg)
 mux:=http.NewServeMux();mux.HandleFunc("POST /api/login",a.Login);mux.HandleFunc("POST /api/logout",a.Logout);mux.HandleFunc("POST /api/conversations/{id}/messages",a.Middleware(s.SendMessage));server:=httptest.NewServer(mux);defer server.Close()
 login,_:=http.Post(server.URL+"/api/login","application/json",strings.NewReader(fmt.Sprintf(`{"username":"student_a1","password":%q}`,cfg.SeedStudentPassword)))
 var token struct{Token string `json:"token"`};json.NewDecoder(login.Body).Decode(&token);login.Body.Close();if token.Token==""{t.Fatal("login failed")};defer func(){req,_:=http.NewRequest("POST",server.URL+"/api/logout",nil);req.Header.Set("Authorization","Bearer "+token.Token);res,_:=http.DefaultClient.Do(req);if res!=nil{res.Body.Close()}}()
 res,e:=d.Exec(`INSERT INTO tutor_agents(class_id,name,prompt,enabled) VALUES(1,?,'角色验收',TRUE)`,fmt.Sprintf("测试%d",time.Now().UnixNano()));if e!=nil{t.Fatal(e)};aid,_:=res.LastInsertId();conv:=[]int64{}
 var uid int64;d.QueryRow(`SELECT id FROM users WHERE username='student_a1'`).Scan(&uid)
 for i:=0;i<4;i++{res,e=d.Exec(`INSERT INTO conversations(class_id,user_id,agent_id) VALUES(1,?,?)`,uid,aid);if e!=nil{t.Fatal(e)};id,_:=res.LastInsertId();conv=append(conv,id)}
 t.Cleanup(func(){for _,id:=range conv{d.Exec(`DELETE FROM conversations WHERE id=?`,id)};d.Exec(`DELETE FROM tutor_agents WHERE id=?`,aid)})
 send:=func(ctx context.Context,id int64,q string)*http.Response{req,_:=http.NewRequestWithContext(ctx,"POST",fmt.Sprintf("%s/api/conversations/%d/messages",server.URL,id),bytes.NewBufferString(fmt.Sprintf(`{"content":%q}`,q)));req.Header.Set("Authorization","Bearer "+token.Token);resp,e:=http.DefaultClient.Do(req);if e!=nil{t.Fatal(e)};return resp}
 one:=send(context.Background(),conv[0],"parallel one");two:=send(context.Background(),conv[1],"parallel two");defer one.Body.Close();defer two.Body.Close()
 // Both streams must reach meta while the mock is still blocked.
 for _,r:=range []*http.Response{one,two}{scanner:=bufio.NewScanner(r.Body);if !scanner.Scan()||scanner.Text()!="event: meta"{t.Fatal("no independent meta")}}
 clash:=send(context.Background(),conv[0],"duplicate");if clash.StatusCode!=409{t.Fatal(clash.StatusCode)};clash.Body.Close();close(gate)
 for _,r:=range []*http.Response{one,two}{io.Copy(io.Discard,r.Body)}
 for _,id:=range conv[:2]{var text,status string;if e=d.QueryRow(`SELECT content,status FROM messages WHERE conversation_id=? AND role='assistant'`,id).Scan(&text,&status);e!=nil||text!="部分正文"||status!="complete"{t.Fatal(text,status,e)}}
 failed:=send(context.Background(),conv[2],"failure test");b,_:=io.ReadAll(failed.Body);failed.Body.Close();if !strings.Contains(string(b),"event: error")||strings.Contains(string(b),"[9]"){t.Fatal(string(b))}
 var status string;if e=d.QueryRow(`SELECT status FROM messages WHERE conversation_id=? AND role='assistant'`,conv[2]).Scan(&status);e!=nil||status!="partial"{t.Fatal(status,e)}
 ctx,cancel:=context.WithCancel(context.Background());stopped:=send(ctx,conv[3],"cancel test");scan:=bufio.NewScanner(stopped.Body);for scan.Scan(){if scan.Text()=="event: delta"{break}};cancel();stopped.Body.Close()
 deadline:=time.Now().Add(3*time.Second);for{e=d.QueryRow(`SELECT status FROM messages WHERE conversation_id=? AND role='assistant'`,conv[3]).Scan(&status);if e==nil&&status=="partial"{break};if time.Now().After(deadline){t.Fatal("cancelled text was not persisted",e)};time.Sleep(20*time.Millisecond)}
}
