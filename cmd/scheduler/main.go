package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"github.com/nekoimi/go-project-template/internal/app"
	"github.com/nekoimi/go-project-template/internal/scheduler"
)

func main() {
	configPath := flag.String("config", "configs/config.dev.yaml", "path to config file")
	flag.Parse()

	// 复用 app 包的初始化逻辑
	app, cleanup, err := app.Initialize(*configPath)
	if err != nil {
		log.Fatalf("failed to initialize: %v", err)
	}
	defer cleanup()

	// 使用 app 中已经初始化好的 Scheduler
	sched := app.Scheduler
	if sched == nil {
		sched = scheduler.New(app.Config.Scheduler, app.Logger, app.DB)
		sched.RegisterJobs()
	}

	sched.Start()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	app.Logger.Info("shutting down scheduler")
	sched.Stop()
}
