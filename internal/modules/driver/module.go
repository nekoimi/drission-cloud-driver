package driver

import (
	"context"

	"github.com/nekoimi/drission-cloud-driver/internal/framework"
	"github.com/nekoimi/drission-cloud-driver/internal/module"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/response"
)

const ModuleName = "driver"

func init() {
	module.Register(&driverModule{}, module.ScopeHTTP)
}

type driverModule struct{}

func (m *driverModule) Name() string {
	return ModuleName
}

func (m *driverModule) Register(ctx *framework.ModuleContext) error {
	if !ctx.ModuleEnabled(ModuleName) {
		return nil
	}

	sys := newSystemHandler(ctx.DriverRegistry, ctx.Logger)
	h := newDriverHandler(ctx.DriverRegistry, ctx.BrowserManager, ctx.OfflineStore, ctx.Logger)
	log := ctx.Logger

	// Driver API
	api := ctx.Router.Group("/drivers")
	{
		api.GET("", response.Handle(sys.ListDrivers, log))

		d := api.Group("/:platform")
		{
			d.GET("/capabilities", response.Handle(h.GetCapabilities, log))

			// Offline download
			offlineApi := d.Group("/offline")
			{
				offlineApi.POST("/add", response.Handle(h.AddOfflineTask, log))
				offlineApi.GET("/tasks", response.Handle(h.ListOfflineTasks, log))
				offlineApi.GET("/tasks/:id", response.Handle(h.GetOfflineTask, log))
				offlineApi.DELETE("/tasks/:id", response.Handle(h.RemoveOfflineTask, log))
			}

			// File system
			fs := d.Group("/fs")
			{
				fs.POST("/mkdir", response.Handle(h.Mkdir, log))
				fs.DELETE("/remove", response.Handle(h.Remove, log))
				fs.POST("/move", response.Handle(h.Move, log))
				fs.POST("/rename", response.Handle(h.Rename, log))
				fs.GET("/list", response.Handle(h.List, log))
				fs.GET("/search", response.Handle(h.Search, log))
				fs.GET("/dirname2cid", response.Handle(h.DirName2CID, log))
			}

			// Media
			media := d.Group("/media")
			{
				media.GET("/url", response.Handle(h.GetDownloadURL, log))
			}
		}
	}

	ctx.Logger.Info("driver module registered")
	return nil
}

func (m *driverModule) Shutdown(_ context.Context, moduleCtx *framework.ModuleContext) error {
	if moduleCtx.DriverRegistry != nil {
		return moduleCtx.DriverRegistry.Close()
	}
	return nil
}
