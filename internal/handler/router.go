package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/nekoimi/go-project-template/internal/config"
	v1 "github.com/nekoimi/go-project-template/internal/handler/v1"
	"github.com/nekoimi/go-project-template/internal/handler/middleware"
	"github.com/nekoimi/go-project-template/internal/repository"
	"github.com/nekoimi/go-project-template/internal/service"
	"github.com/nekoimi/go-project-template/internal/storage"
	ws "github.com/nekoimi/go-project-template/internal/websocket"
)

func SetupRouter(cfg *config.Config, logger *zap.Logger, db *gorm.DB, fileStorage storage.FileStorage, wsManager *ws.Manager) *gin.Engine {
	gin.SetMode(cfg.Server.Mode)
	r := gin.New()

	// Middleware
	r.Use(middleware.Recovery(logger))
	r.Use(middleware.RequestID())
	r.Use(middleware.RequestLogger(logger))
	r.Use(middleware.CORS(cfg.Server.AllowedOrigins))

	// Rate limiting
	if cfg.RateLimit.Enabled {
		r.Use(middleware.RateLimit(cfg.RateLimit.RPS, cfg.RateLimit.Burst))
	}

	// Health check (liveness)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Readiness check (DB ping)
	r.GET("/ready", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil || sqlDB.Ping() != nil {
			c.JSON(503, gin.H{"status": "not ready"})
			return
		}
		c.JSON(200, gin.H{"status": "ready"})
	})

	// Swagger
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Local file serving
	if cfg.Storage.Driver == "local" {
		r.Static("/uploads", cfg.Storage.Local.UploadDir)
	}

	// Repositories
	userRepo := repository.NewUserRepository(db)

	// Services
	jwtExpire := time.Duration(cfg.JWT.ExpireHours) * time.Hour
	authService := service.NewAuthService(userRepo, db, cfg.JWT.Secret, jwtExpire)
	userService := service.NewUserService(userRepo)
	fileService := service.NewFileService(fileStorage, cfg.Storage.Local.AllowedExts, cfg.Storage.Local.AllowedMIMEs)

	// Handlers
	authHandler := v1.NewAuthHandler(authService, logger)
	userHandler := v1.NewUserHandler(userService, logger)
	uploadHandler := v1.NewUploadHandler(fileService, logger)

	// API v1 routes
	api := r.Group("/v1")
	{
		// Auth (public)
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
		}

		// Protected routes
		protected := api.Group("")
		protected.Use(middleware.JWTAuth(cfg.JWT.Secret))
		{
			// Users
			users := protected.Group("/users")
			{
				users.GET("/profile", userHandler.GetProfile)
			}

			// Upload
			upload := protected.Group("/upload")
			{
				upload.POST("/single", uploadHandler.UploadSingle)
				upload.POST("/multiple", uploadHandler.UploadMultiple)
			}
		}
	}

	if cfg.Websocket.Enabled {
		wsHandler := v1.NewWSHandler(ws.NewWSHandler(wsManager, cfg.JWT.Secret, logger, cfg.Server.AllowedOrigins, cfg.Websocket))
		r.GET("/ws/v1/chat", wsHandler.Upgrade)
	}

	return r
}
