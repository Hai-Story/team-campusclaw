// Explicit local import of original junior-school examples. No server startup side effects.
package main

import (
	"campusclaw/backend/internal/auth"
	"campusclaw/backend/internal/config"
	"campusclaw/backend/internal/curriculum"
	"campusclaw/backend/internal/db"
	"campusclaw/backend/internal/retrieval"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

func run() error {
	teacher := flag.String("teacher", "", "目标教师用户名（必填）")
	homework := flag.Bool("homework", false, "复制已有题库为九科示例作业，不调用模型")
	apply := flag.Bool("apply", false, "显式写入原创示例；默认仅校验目录")
	flag.Parse()
	units, e := curriculum.JuniorUnits()
	if e != nil {
		return e
	}
	if !*apply {
		fmt.Printf("目录校验通过：%d个主题，九科。加-apply -teacher指定目标后写入。\n", len(units))
		return nil
	}
	if *teacher == "" {
		return fmt.Errorf("必须指定teacher")
	}
	cfg, e := config.Load()
	if e != nil {
		return e
	}
	d, e := db.Open(cfg)
	if e != nil {
		return e
	}
	defer d.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	var u auth.User
	if e = d.QueryRowContext(ctx, `SELECT id,username,role,class_id FROM users WHERE username=? AND role='teacher'`, *teacher).Scan(&u.ID, &u.Username, &u.Role, &u.ClassID); e != nil {
		return fmt.Errorf("目标必须是现有教师")
	}
	if e = curriculum.Migrate(d); e != nil {
		return e
	}
	if *homework {
		result, err := (&curriculum.Store{DB: d}).ImportJuniorHomework(ctx, u)
		b, _ := json.Marshal(result)
		fmt.Println(string(b))
		return err
	}
	q := retrieval.NewQdrantClient(cfg)
	ix := &retrieval.Indexer{DB: d, Cfg: cfg, Embed: &retrieval.EmbeddingClient{BaseURL: cfg.EmbeddingBaseURL, APIKey: cfg.EmbeddingAPIKey, Model: cfg.EmbeddingModel, Dim: cfg.EmbeddingDim}, Qdr: q}
	result, e := (&curriculum.Store{DB: d}).ImportJunior(ctx, u, cfg.UploadDir, ix)
	b, _ := json.Marshal(result)
	fmt.Println(string(b))
	return e
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
