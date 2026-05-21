package main

// @title           Go Template API
// @version         1.0
// @description     A Go backend template project.
// @host            localhost:8080
// @BasePath        /v1
// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/nekoimi/go-project-template/internal/app"
)

func main() {
	configPath := flag.String("config", "configs/config.dev.yaml", "path to config file")
	flag.Parse()

	a, cleanup, err := app.Initialize(*configPath)
	if err != nil {
		log.Fatalf("failed to initialize app: %v", err)
	}

	if a.Config.Websocket.Enabled {
		go a.WSManager.Run()
	}

	// Start scheduler if enabled
	if a.Scheduler != nil {
		a.Scheduler.Start()
	}

	// HTTP server
	srv := &http.Server{
		Addr:    ":" + a.Config.Server.Port,
		Handler: a.Engine,
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		a.Logger.Info("server starting", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.Logger.Fatal("server failed", zap.Error(err))
		}
	}()

	<-ctx.Done()
	a.Logger.Info("shutting down server")

	timeout := time.Duration(a.Config.Server.ShutdownTimeout) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 1. HTTP shutdown
	if err := srv.Shutdown(shutdownCtx); err != nil {
		a.Logger.Error("server shutdown error", zap.Error(err))
	}

	// 2. Stop scheduler
	if a.Scheduler != nil {
		a.Scheduler.Stop()
	}

	if a.Config.Websocket.Enabled {
		a.WSManager.Shutdown()
	}

	// Cleanup resources
	cleanup()

	a.Logger.Info("server stopped")
}
