// CampusClaw api 入口：加载配置（缺失即退出）→ 等待数据库就绪 → 建表种子 → 注册路由。
// 信任边界在此：认证、授权、班级隔离、上传校验全部发生在本进程（design.md D1）。
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"campusclaw/backend/internal/apidocs"
	"campusclaw/backend/internal/auth"
	"campusclaw/backend/internal/config"
	"campusclaw/backend/internal/db"
	"campusclaw/backend/internal/materials"
	"campusclaw/backend/internal/mcp"
	"campusclaw/backend/internal/retrieval"
	"campusclaw/backend/internal/runtimehttp"
	"campusclaw/backend/internal/tutor"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("配置错误：%v", err) // 失败即停，禁止内置默认值（spec R7）
	}
	d, err := db.Open(cfg)
	if err != nil {
		log.Fatalf("数据库配置错误：%v", err)
	}
	mcpSvc, err := mcp.FromEnv(d)
	if err != nil {
		log.Fatalf("MCP配置错误：%v", err)
	}
	if err := db.WaitReady(d); err != nil {
		log.Fatalf("%v", err)
	}
	if err := db.Migrate(d); err != nil {
		log.Fatalf("建表失败：%v", err)
	}
	if err := db.Seed(d, cfg); err != nil {
		log.Fatalf("种子失败：%v", err)
	}

	if err := tutor.Migrate(d); err != nil {
		log.Fatalf("助手建表失败：%v", err)
	}
	if err := mcp.Migrate(d); err != nil {
		log.Fatalf("MCP建表失败：%v", err)
	}
	if err := tutor.Seed(d); err != nil {
		log.Fatalf("助手种子失败：%v", err)
	}

	authSvc := auth.NewService(d, cfg)
	matSvc := materials.NewService(d, cfg)

	// 检索链路（嵌入/对话网关 + Qdrant）。
	// Qdrant 集合创建失败不阻断启动——keyword 模式仍可服务，vector/hybrid 按 D8 返回 503。
	embed := &retrieval.EmbeddingClient{BaseURL: cfg.EmbeddingBaseURL, APIKey: cfg.EmbeddingAPIKey, Model: cfg.EmbeddingModel, Dim: cfg.EmbeddingDim}
	chat := &retrieval.ChatClient{BaseURL: cfg.ChatBaseURL, APIKey: cfg.ChatAPIKey, Model: cfg.ChatModel}
	qdr := retrieval.NewQdrantClient(cfg)
	qdrCtx, qdrCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := qdr.EnsureCollection(qdrCtx); err != nil {
		log.Printf("警告：Qdrant 集合创建失败（keyword 模式仍可用，vector/hybrid 将返回 503）：%v", err)
		qdr.ResetEnsure() // 允许后续请求时再试
	}
	qdrCancel()
	retrievalSvc := retrieval.NewService(d, cfg, embed, qdr, chat)
	matSvc.SetIndexer(retrievalSvc.Indexer)
	// 种子材料与既有材料按 auto 策略补齐索引（嵌入失败标 failed，绝不阻断启动）。
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		retrievalSvc.Indexer.BackfillMissing(ctx)
	}()

	// dbGuard：数据库不可用时业务接口返回 503，不得误判为会话失效 401（design.md D9）。
	dbGuard := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !db.Ready(d) {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"error":"服务暂时不可用"}`))
				return
			}
			next(w, r)
		}
	}

	mux := http.NewServeMux()
	apidocs.Mount(mux)
	runtimehttp.Probes(mux, d, cfg.Version, cfg.QdrantURL, strings.Split(os.Getenv("MCP_ALLOWED_BASE_URLS"), ",")[0])

	mux.HandleFunc("POST /api/login", dbGuard(authSvc.Login))
	mux.HandleFunc("POST /api/logout", dbGuard(authSvc.Logout))
	mux.HandleFunc("GET /api/me", dbGuard(authSvc.Middleware(authSvc.Me)))
	mux.HandleFunc("GET /api/materials", dbGuard(authSvc.Middleware(matSvc.List)))
	mux.HandleFunc("GET /api/materials/{id}", dbGuard(authSvc.Middleware(matSvc.Detail)))
	mux.HandleFunc("GET /api/materials/{id}/file", dbGuard(authSvc.Middleware(matSvc.File)))
	mux.HandleFunc("PUT /api/materials/{id}", dbGuard(authSvc.Middleware(auth.RequireTeacher(matSvc.Update))))
	mux.HandleFunc("DELETE /api/materials/{id}", dbGuard(authSvc.Middleware(auth.RequireTeacher(matSvc.Delete))))
	mux.HandleFunc("POST /api/materials", dbGuard(authSvc.Middleware(auth.RequireTeacher(matSvc.Upload))))

	// 检索、问答与索引重建（均在 dbGuard 与会话中间件之后，design.md 接口一览）
	mux.HandleFunc("POST /api/search", dbGuard(authSvc.Middleware(retrievalSvc.Search)))
	mux.HandleFunc("POST /api/ask", dbGuard(authSvc.Middleware(retrievalSvc.Ask)))
	mux.HandleFunc("POST /api/materials/{id}/reindex", dbGuard(authSvc.Middleware(auth.RequireTeacher(retrievalSvc.Reindex))))

	tutorSvc := tutor.NewService(d, retrievalSvc.Searcher, chat)
	tutorSvc.MCP = mcpSvc
	mux.HandleFunc("PUT /api/ai-apps/{key}/template", dbGuard(authSvc.Middleware(auth.RequireTeacher(tutorSvc.SaveAppTemplate))))
	mux.HandleFunc("GET /api/ai-apps", dbGuard(authSvc.Middleware(tutorSvc.Apps)))
	mux.HandleFunc("POST /api/ai-apps/{key}/launch", dbGuard(authSvc.Middleware(auth.RequireStudent(tutorSvc.LaunchApp))))
	mux.HandleFunc("GET /api/agents", dbGuard(authSvc.Middleware(tutorSvc.Agents)))
	mux.HandleFunc("GET /api/agents/{id}", dbGuard(authSvc.Middleware(tutorSvc.Agent)))
	mux.HandleFunc("POST /api/agents", dbGuard(authSvc.Middleware(auth.RequireTeacher(tutorSvc.SaveAgent))))
	mux.HandleFunc("DELETE /api/agents/{id}", dbGuard(authSvc.Middleware(auth.RequireTeacher(tutorSvc.DeleteAgent))))
	mux.HandleFunc("PUT /api/agents/{id}", dbGuard(authSvc.Middleware(auth.RequireTeacher(tutorSvc.SaveAgent))))
	mux.HandleFunc("GET /api/skills", dbGuard(authSvc.Middleware(tutorSvc.Skills)))
	mux.HandleFunc("GET /api/skills/{id}", dbGuard(authSvc.Middleware(tutorSvc.Skill)))
	mux.HandleFunc("POST /api/skills/import", dbGuard(authSvc.Middleware(auth.RequireTeacher(tutorSvc.ImportSkill))))
	mux.HandleFunc("POST /api/skills", dbGuard(authSvc.Middleware(auth.RequireTeacher(tutorSvc.SaveSkill))))
	mux.HandleFunc("PUT /api/skills/{id}", dbGuard(authSvc.Middleware(auth.RequireTeacher(tutorSvc.SaveSkill))))
	mux.HandleFunc("DELETE /api/skills/{id}", dbGuard(authSvc.Middleware(auth.RequireTeacher(tutorSvc.SaveSkill))))
	mux.HandleFunc("GET /api/conversations", dbGuard(authSvc.Middleware(auth.RequireStudent(tutorSvc.Conversations))))
	mux.HandleFunc("POST /api/conversations", dbGuard(authSvc.Middleware(auth.RequireStudent(tutorSvc.CreateConversation))))
	mux.HandleFunc("DELETE /api/conversations/{id}", dbGuard(authSvc.Middleware(auth.RequireStudent(tutorSvc.DeleteConversation))))
	mux.HandleFunc("GET /api/conversations/{id}", dbGuard(authSvc.Middleware(auth.RequireStudent(tutorSvc.Conversation))))
	mux.HandleFunc("GET /api/conversations/{id}/messages/{message}/trace", dbGuard(authSvc.Middleware(auth.RequireStudent(tutorSvc.MessageTrace))))
	mux.HandleFunc("POST /api/conversations/{id}/messages", dbGuard(authSvc.Middleware(auth.RequireStudent(tutorSvc.SendMessage))))

	mux.HandleFunc("GET /api/mcp/servers", dbGuard(authSvc.Middleware(auth.RequireTeacher(mcpSvc.ListServers))))
	mux.HandleFunc("POST /api/mcp/servers", dbGuard(authSvc.Middleware(auth.RequireTeacher(mcpSvc.SaveServer))))
	mux.HandleFunc("GET /api/mcp/servers/{id}", dbGuard(authSvc.Middleware(auth.RequireTeacher(mcpSvc.GetServer))))
	mux.HandleFunc("PUT /api/mcp/servers/{id}", dbGuard(authSvc.Middleware(auth.RequireTeacher(mcpSvc.SaveServer))))
	mux.HandleFunc("DELETE /api/mcp/servers/{id}", dbGuard(authSvc.Middleware(auth.RequireTeacher(mcpSvc.DeleteServer))))
	mux.HandleFunc("POST /api/mcp/servers/{id}/connect", dbGuard(authSvc.Middleware(auth.RequireTeacher(mcpSvc.ConnectServer))))
	mux.HandleFunc("POST /api/mcp/servers/{id}/disconnect", dbGuard(authSvc.Middleware(auth.RequireTeacher(mcpSvc.DisconnectServer))))
	mux.HandleFunc("GET /api/mcp/servers/{id}/tools", dbGuard(authSvc.Middleware(auth.RequireTeacher(mcpSvc.ListTools))))
	mux.HandleFunc("POST /api/mcp/servers/{id}/tools/{tool}/approve", dbGuard(authSvc.Middleware(auth.RequireTeacher(mcpSvc.ApproveTool))))
	mux.HandleFunc("GET /api/mcp/tool-calls", dbGuard(authSvc.Middleware(auth.RequireTeacher(mcpSvc.Calls))))

	log.Printf("CampusClaw api 监听 :%s", cfg.Port)
	server := &http.Server{Addr: ":" + cfg.Port, Handler: runtimehttp.Middleware(mux), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(shutdown)
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
