package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"campusclaw/backend/internal/config"
	"campusclaw/backend/internal/db"
	"campusclaw/backend/internal/grading"
)

func main() {
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	grading.RunRelay(ctx, &grading.Store{DB: d}, settings)
}
