package framework

import (
	"context"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/browser"
	"github.com/nekoimi/drission-cloud-driver/internal/config"
	"github.com/nekoimi/drission-cloud-driver/internal/drivers"
	"github.com/nekoimi/drission-cloud-driver/internal/offline"
)

type RouterContext struct {
	Engine *gin.Engine
	API    *gin.RouterGroup
}

type ModuleContext struct {
	Config         *config.Config
	Logger         *zap.Logger
	Router         *gin.Engine
	API            *gin.RouterGroup
	BrowserManager *browser.Manager
	DriverRegistry *drivers.Registry
	OfflineStore   offline.Store
	Health         *HealthRegistry
	Events         *EventBus
}

func NewModuleContext(
	cfg *config.Config,
	logger *zap.Logger,
	router *RouterContext,
	browserMgr *browser.Manager,
	driverRegistry *drivers.Registry,
	offlineStore offline.Store,
	health *HealthRegistry,
	events *EventBus,
) *ModuleContext {
	var engine *gin.Engine
	var api *gin.RouterGroup
	if router != nil {
		engine = router.Engine
		api = router.API
	}

	return &ModuleContext{
		Config:         cfg,
		Logger:         logger,
		Router:         engine,
		API:            api,
		BrowserManager: browserMgr,
		DriverRegistry: driverRegistry,
		OfflineStore:   offlineStore,
		Health:         health,
		Events:         events,
	}
}

func (ctx *ModuleContext) ModuleEnabled(name string) bool {
	if ctx == nil || ctx.Config == nil {
		return false
	}
	return ctx.Config.ModuleEnabled(name)
}

func (ctx *ModuleContext) AddHealthCheck(name string, check HealthCheck) {
	if ctx == nil || ctx.Health == nil {
		return
	}
	ctx.Health.Register(name, check)
}

func (ctx *ModuleContext) Subscribe(topic string, handler EventHandler) {
	if ctx == nil || ctx.Events == nil {
		return
	}
	ctx.Events.Subscribe(topic, handler)
}

func (ctx *ModuleContext) Publish(publishCtx context.Context, event Event) error {
	if ctx == nil || ctx.Events == nil {
		return nil
	}
	return ctx.Events.Publish(publishCtx, event)
}
