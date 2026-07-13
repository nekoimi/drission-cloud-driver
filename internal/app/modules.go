package app

import (
	"github.com/nekoimi/drission-cloud-driver/internal/framework"
	"github.com/nekoimi/drission-cloud-driver/internal/module"

	_ "github.com/nekoimi/drission-cloud-driver/internal/modules/browser"
	_ "github.com/nekoimi/drission-cloud-driver/internal/modules/driver"
)

func registeredModules() []framework.Module {
	return module.Modules(module.ScopeHTTP, module.ScopeScheduler)
}
