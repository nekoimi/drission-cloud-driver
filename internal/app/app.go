package app

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/nekoimi/go-project-template/internal/config"
	"github.com/nekoimi/go-project-template/internal/handler"
	"github.com/nekoimi/go-project-template/internal/infrastructure/database"
	"github.com/nekoimi/go-project-template/internal/infrastructure/logger"
	"github.com/nekoimi/go-project-template/internal/pkg/snowflake"
	"github.com/nekoimi/go-project-template/internal/pkg/timeutil"
	"github.com/nekoimi/go-project-template/internal/scheduler"
	"github.com/nekoimi/go-project-template/internal/storage"
	"github.com/nekoimi/go-project-template/internal/storage/local"
	"github.com/nekoimi/go-project-template/internal/storage/minio"
	ws "github.com/nekoimi/go-project-template/internal/websocket"
)

type App struct {
	Engine   *gin.Engine
	Config   *config.Config
	Logger   *zap.Logger
	DB       *gorm.DB
	Storage  storage.FileStorage
	WSManager *ws.Manager
	Scheduler *scheduler.Scheduler
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

	// 3. Snowflake
	if err := snowflake.Init(cfg.Snowflake.NodeID); err != nil {
		return nil, nil, fmt.Errorf("failed to init snowflake: %w", err)
	}

	// 4. Logger
	log, err := logger.NewLogger(cfg.Server.Mode)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create logger: %w", err)
	}

	// 5. Database
	db, err := database.NewPostgresDB(cfg.Database, log, cfg.Server.Mode)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect database: %w", err)
	}

	// 6. Storage
	var fileStorage storage.FileStorage
	switch cfg.Storage.Driver {
	case "minio":
		fileStorage, err = minio.New(cfg.Storage)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create minio storage: %w", err)
		}
	default:
		fileStorage = local.New(cfg.Storage)
	}

	// 7. WebSocket manager
	wsManager := ws.NewManager(log)

	// 8. Setup router
	router := handler.SetupRouter(cfg, log, db, fileStorage, wsManager)

	// 9. Scheduler (optional)
	var sched *scheduler.Scheduler
	if cfg.Scheduler.Enabled {
		sched = scheduler.New(cfg.Scheduler, log, db)
		sched.RegisterJobs()
	}

	app := &App{
		Engine:    router,
		Config:    cfg,
		Logger:    log,
		DB:        db,
		Storage:   fileStorage,
		WSManager: wsManager,
		Scheduler: sched,
	}

	cleanup := func() {
		log.Info("cleaning up resources")
		if sqlDB, err := db.DB(); err == nil {
			if cerr := sqlDB.Close(); cerr != nil {
				log.Warn("failed to close database", zap.Error(cerr))
			}
		}
		_ = log.Sync()
	}

	return app, cleanup, nil
}
