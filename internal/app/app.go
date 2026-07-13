package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/browser"
	"github.com/nekoimi/drission-cloud-driver/internal/cloak"
	"github.com/nekoimi/drission-cloud-driver/internal/config"
	"github.com/nekoimi/drission-cloud-driver/internal/drivers"
	"github.com/nekoimi/drission-cloud-driver/internal/drivers/pan115"
	"github.com/nekoimi/drission-cloud-driver/internal/framework"
	"github.com/nekoimi/drission-cloud-driver/internal/handler"
	"github.com/nekoimi/drission-cloud-driver/internal/offline"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/logger"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/timeutil"
)

type App struct {
	Engine         *gin.Engine
	Config         *config.Config
	Logger         *zap.Logger
	BrowserManager *browser.Manager
	DriverRegistry *drivers.Registry
	OfflineStore   offline.Store
	HTTPServer     *http.Server
	httpErr        chan error
	Modules        []framework.Module
	ModuleCtx      *framework.ModuleContext
}

func (a *App) Boot(ctx context.Context) error {
	if a == nil {
		return nil
	}
	return framework.BootModules(ctx, a.ModuleCtx, a.Modules...)
}

func (a *App) Shutdown(ctx context.Context) error {
	if a == nil {
		return nil
	}
	return framework.ShutdownModules(ctx, a.ModuleCtx, a.Modules...)
}

func Initialize(configPath string) (*App, func(), error) {
	return initialize(configPath, registeredModules())
}

func initialize(configPath string, modules []framework.Module) (*App, func(), error) {
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

	// Register drivers based on configuration
	for _, platform := range cfg.Drivers.Platforms {
		switch platform {
		case "115":
			registry.Register("115", pan115.NewFactoryWithDirIDCacheDSN(cfg.Offline.Store.DSN))
			// Add more platforms here
			// case "pikpak":
			//     registry.Register("pikpak", pikpak.NewFactory())
		}
	}

	// 7. Offline task store
	offlineStore, err := newOfflineStore(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create offline store: %w", err)
	}

	// 8. Health checks
	health := framework.NewHealthRegistry()
	events := framework.NewEventBus()

	// 9. Setup base router
	router := handler.SetupRouter(cfg, log, health)

	// 10. Register feature modules
	moduleCtx := framework.NewModuleContext(cfg, log, router, browserMgr, registry, offlineStore, health, events)
	if err := framework.RegisterModules(moduleCtx, modules...); err != nil {
		return nil, nil, err
	}

	app := &App{
		Engine:         router.Engine,
		Config:         cfg,
		Logger:         log,
		BrowserManager: browserMgr,
		DriverRegistry: registry,
		OfflineStore:   offlineStore,
		Modules:        modules,
		ModuleCtx:      moduleCtx,
	}

	cleanup := func() {
		log.Info("cleaning up resources")
		if closer, ok := offlineStore.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				log.Warn("failed to close offline store", zap.Error(err))
			}
		}
		_ = log.Sync()
	}

	return app, cleanup, nil
}

func newOfflineStore(cfg *config.Config) (offline.Store, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Offline.Store.Driver)) {
	case "", "postgres", "postgresql", "pg":
		return offline.NewPostgresStore(cfg.Offline.Store.DSN)
	case "sqlite":
		return offline.NewSQLiteStore(cfg.Offline.Store.DSN)
	case "memory":
		return offline.NewMemoryStore(), nil
	default:
		return nil, fmt.Errorf("unsupported offline store driver: %s", cfg.Offline.Store.Driver)
	}
}
