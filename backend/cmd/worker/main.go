package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"campusclaw/backend/internal/config"
	"campusclaw/backend/internal/db"
	"campusclaw/backend/internal/grading"
)

func main() {
	publish := flag.String("publish", "", "publish an existing task ID for controlled replay")
	migrate := flag.Bool("migrate", false, "repeat the idempotent migration")
	metrics := flag.Bool("metrics", false, "read task counts")
	queues := flag.Bool("queues", false, "read queue lengths")
	refresh := flag.Bool("refresh-legacy", false, "replace up to 20 legacy/test verdicts using the configured model, retaining prior results")
	flag.Parse()
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	settings, err := grading.LoadSettings()
	if err != nil {
		log.Fatal(err)
	}
	d, err := db.Open(cfg)
	if err != nil {
		log.Fatal("数据库配置错误")
	}
	defer d.Close()
	if db.WaitReady(d) != nil {
		log.Fatal("数据库不可用")
	}
	store := &grading.Store{DB: d}
	if *refresh {
		count, err := store.RefreshLegacy(context.Background(), settings)
		if err != nil {
			log.Fatal(err)
		}
		json.NewEncoder(os.Stdout).Encode(map[string]int{"updated": count})
		return
	}
	if *queues {
		broker, err := grading.OpenBroker(settings.AMQPURL)
		if err != nil {
			log.Fatal("消息代理不可用")
		}
		defer broker.Close()
		counts, err := broker.QueueStats()
		if err != nil {
			log.Fatal("队列指标不可用")
		}
		json.NewEncoder(os.Stdout).Encode(counts)
		return
	}
	if *migrate {
		if grading.Migrate(d) != nil {
			log.Fatal("迁移失败")
		}
		return
	}
	if *metrics {
		counts, err := store.Metrics(context.Background())
		if err != nil {
			log.Fatal("指标不可用")
		}
		json.NewEncoder(os.Stdout).Encode(counts)
		return
	}
	if *publish != "" {
		if len(*publish) != 32 {
			log.Fatal("任务标识格式不合法")
		}
		broker, err := grading.OpenBroker(settings.AMQPURL)
		if err != nil {
			log.Fatal("消息代理不可用")
		}
		defer broker.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if broker.Send(ctx, grading.Message{TaskID: *publish, Kind: "work"}) != nil {
			log.Fatal("消息未确认")
		}
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	grading.RunWorker(ctx, store, settings)
}
