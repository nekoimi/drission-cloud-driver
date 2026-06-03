package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/browser"
	"github.com/nekoimi/drission-cloud-driver/internal/config"
	"github.com/nekoimi/drission-cloud-driver/internal/drivers"
	"github.com/nekoimi/drission-cloud-driver/internal/handler/middleware"
	v1 "github.com/nekoimi/drission-cloud-driver/internal/handler/v1"
	"github.com/nekoimi/drission-cloud-driver/internal/offline"
)

func SetupRouter(cfg *config.Config, logger *zap.Logger, browserMgr *browser.Manager, registry *drivers.Registry, offlineStore offline.Store) *gin.Engine {
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

	// Handlers
	systemHandler := v1.NewSystemHandler(registry, logger)
	profileHandler := v1.NewProfileHandler(browserMgr, logger)
	driverHandler := v1.NewDriverHandler(registry, browserMgr, offlineStore, logger)

	// Health check
	r.GET("/health", systemHandler.Health)

	// Profile management
	profiles := r.Group("/profiles")
	{
		profiles.GET("", profileHandler.ListProfiles)
		profiles.GET("/:id", profileHandler.GetProfile)
		profiles.POST("/:id/start", profileHandler.StartProfile)
		profiles.POST("/:id/stop", profileHandler.StopProfile)
	}

	// Driver API
	api := r.Group("/drivers")
	{
		api.GET("", systemHandler.ListDrivers)

		d := api.Group("/:platform")
		{
			d.GET("/capabilities", driverHandler.GetCapabilities)

			// Offline download
			offlineApi := d.Group("/offline")
			{
				offlineApi.POST("/add", driverHandler.AddOfflineTask)
				offlineApi.GET("/tasks", driverHandler.ListOfflineTasks)
				offlineApi.GET("/tasks/:id", driverHandler.GetOfflineTask)
				offlineApi.DELETE("/tasks/:id", driverHandler.RemoveOfflineTask)
			}

			// File system
			fs := d.Group("/fs")
			{
				fs.POST("/mkdir", driverHandler.Mkdir)
				fs.DELETE("/remove", driverHandler.Remove)
				fs.POST("/move", driverHandler.Move)
				fs.POST("/rename", driverHandler.Rename)
				fs.GET("/list", driverHandler.List)
				fs.GET("/search", driverHandler.Search)
				fs.GET("/dirname2cid", driverHandler.DirName2CID)
			}

			// Media
			media := d.Group("/media")
			{
				media.GET("/url", driverHandler.GetDownloadURL)
			}
		}
	}

	return r
}
