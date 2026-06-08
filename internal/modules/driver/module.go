package driver

import (
	"context"

	"github.com/nekoimi/drission-cloud-driver/internal/framework"
	"github.com/nekoimi/drission-cloud-driver/internal/module"
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

	systemHandler := newSystemHandler(ctx.DriverRegistry, ctx.Logger)
	driverHandler := newDriverHandler(ctx.DriverRegistry, ctx.BrowserManager, ctx.OfflineStore, ctx.Logger)

	// Driver API
	api := ctx.Router.Group("/drivers")
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

	ctx.Logger.Info("driver module registered")
	return nil
}

func (m *driverModule) Shutdown(_ context.Context, moduleCtx *framework.ModuleContext) error {
	if moduleCtx.DriverRegistry != nil {
		return moduleCtx.DriverRegistry.Close()
	}
	return nil
}
