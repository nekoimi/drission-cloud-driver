package app

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

func Run(configPath string) error {
	a, cleanup, err := Initialize(configPath)
	if err != nil {
		return err
	}
	defer cleanup()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := a.Start(ctx); err != nil {
		return err
	}

	if a.httpErr == nil {
		<-ctx.Done()
		return a.Stop(context.Background())
	}

	select {
	case <-ctx.Done():
		return a.Stop(context.Background())
	case err := <-a.httpErr:
		if stopErr := a.Stop(context.Background()); stopErr != nil {
			return stopErr
		}
		return err
	}
}

func (a *App) Start(ctx context.Context) error {
	if a == nil {
		return nil
	}
	if err := a.Boot(ctx); err != nil {
		return fmt.Errorf("boot app: %w", err)
	}
	a.startHTTPServer()
	return nil
}

func (a *App) Stop(ctx context.Context) error {
	if a == nil {
		return nil
	}

	a.Logger.Info("shutting down app")
	timeout := time.Duration(a.Config.Server.ShutdownTimeout) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var shutdownErr error
	if a.HTTPServer != nil {
		if err := a.HTTPServer.Shutdown(shutdownCtx); err != nil {
			a.Logger.Error("http server shutdown error", zap.Error(err))
			shutdownErr = err
		}
	}
	if err := a.Shutdown(shutdownCtx); err != nil {
		a.Logger.Error("app shutdown hook error", zap.Error(err))
		if shutdownErr == nil {
			shutdownErr = err
		}
	}
	a.Logger.Info("app stopped")
	return shutdownErr
}

func (a *App) startHTTPServer() {
	a.httpErr = make(chan error, 1)
	a.HTTPServer = &http.Server{
		Addr:    ":" + a.Config.Server.Port,
		Handler: a.Engine,
	}

	go func() {
		a.Logger.Info("http server starting", zap.String("addr", a.HTTPServer.Addr))
		if err := a.HTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.Logger.Error("http server failed", zap.Error(err))
			a.httpErr <- err
		}
	}()
}
