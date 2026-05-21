package app

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/browser"
	"github.com/nekoimi/drission-cloud-driver/internal/cloak"
	"github.com/nekoimi/drission-cloud-driver/internal/config"
	"github.com/nekoimi/drission-cloud-driver/internal/drivers"
	"github.com/nekoimi/drission-cloud-driver/internal/drivers/pan115"
	"github.com/nekoimi/drission-cloud-driver/internal/handler"
	"github.com/nekoimi/drission-cloud-driver/internal/infrastructure/logger"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/timeutil"
)

type App struct {
	Engine         *gin.Engine
	Config         *config.Config
	Logger         *zap.Logger
	BrowserManager *browser.Manager
	Registry       *drivers.Registry
}

func Initialize(configPath string) (*App, func(), error) {
	// 1. Load config
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	// 2. Timezone
	if err := timeutil.SetGlobalLocation(cfg.Server.Timezone); err != nil {
		return nil, nil, fmt.Errorf("failed to set timezone: %w", err)
	}

	// 3. Logger
	log, err := logger.NewLogger(cfg.Server.Mode)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create logger: %w", err)
	}

	// 4. CloakBrowser client
	cloakClient := cloak.NewClient(cfg.Cloak, log)

	// 5. Browser manager
	browserMgr := browser.NewManager(cloakClient, log)

	// 6. Driver registry
	registry := drivers.NewRegistry(log)

	// Register 115 driver (if cookie is configured)
	if cookie, ok := cfg.Drivers.Platforms["115"]; ok {
		if cookieStr, ok := cookie.(string); ok && cookieStr != "" {
			registry.Register("115", pan115.NewFactory(cookieStr))
		}
	}

	// 7. Setup router
	router := handler.SetupRouter(cfg, log, browserMgr, registry)

	app := &App{
		Engine:         router,
		Config:         cfg,
		Logger:         log,
		BrowserManager: browserMgr,
		Registry:       registry,
	}

	cleanup := func() {
		log.Info("cleaning up resources")
		if err := browserMgr.Shutdown(); err != nil {
			log.Warn("failed to shutdown browser manager", zap.Error(err))
		}
		_ = log.Sync()
	}

	return app, cleanup, nil
}
