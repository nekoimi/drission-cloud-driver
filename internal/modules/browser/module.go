package browser

import (
	"context"

	"github.com/nekoimi/drission-cloud-driver/internal/framework"
	"github.com/nekoimi/drission-cloud-driver/internal/module"
)

const ModuleName = "browser"

func init() {
	module.Register(&browserModule{}, module.ScopeHTTP)
}

type browserModule struct{}

func (m *browserModule) Name() string {
	return ModuleName
}

func (m *browserModule) Register(ctx *framework.ModuleContext) error {
	if !ctx.ModuleEnabled(ModuleName) {
		return nil
	}

	handler := newProfileHandler(ctx.BrowserManager, ctx.Logger)

	profiles := ctx.Router.Group("/profiles")
	{
		profiles.GET("", handler.ListProfiles)
		profiles.GET("/:id", handler.GetProfile)
		profiles.POST("/:id/start", handler.StartProfile)
		profiles.POST("/:id/stop", handler.StopProfile)
	}

	ctx.Logger.Info("browser module registered")
	return nil
}

func (m *browserModule) Shutdown(_ context.Context, moduleCtx *framework.ModuleContext) error {
	if moduleCtx.BrowserManager != nil {
		return moduleCtx.BrowserManager.Shutdown()
	}
	return nil
}
