package browser

import (
	"context"

	"github.com/nekoimi/drission-cloud-driver/internal/framework"
	"github.com/nekoimi/drission-cloud-driver/internal/module"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/response"
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

	h := newProfileHandler(ctx.BrowserManager, ctx.Logger)

	profiles := ctx.Router.Group("/profiles")
	{
		profiles.GET("", response.Handle(h.ListProfiles, ctx.Logger))
		profiles.GET("/:id", response.Handle(h.GetProfile, ctx.Logger))
		profiles.POST("/:id/start", response.Handle(h.StartProfile, ctx.Logger))
		profiles.POST("/:id/stop", response.Handle(h.StopProfile, ctx.Logger))
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
