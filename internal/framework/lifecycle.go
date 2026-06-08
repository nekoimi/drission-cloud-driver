package framework

import (
	"context"
	"fmt"
)

type Bootable interface {
	Boot(ctx context.Context, moduleCtx *ModuleContext) error
}

type Shutdownable interface {
	Shutdown(ctx context.Context, moduleCtx *ModuleContext) error
}

func BootModules(ctx context.Context, moduleCtx *ModuleContext, modules ...Module) error {
	for _, module := range modules {
		bootable, ok := module.(Bootable)
		if !ok {
			continue
		}
		if err := bootable.Boot(ctx, moduleCtx); err != nil {
			return fmt.Errorf("boot module %s: %w", module.Name(), err)
		}
	}
	return nil
}

func ShutdownModules(ctx context.Context, moduleCtx *ModuleContext, modules ...Module) error {
	for i := len(modules) - 1; i >= 0; i-- {
		module := modules[i]
		shutdownable, ok := module.(Shutdownable)
		if !ok {
			continue
		}
		if err := shutdownable.Shutdown(ctx, moduleCtx); err != nil {
			return fmt.Errorf("shutdown module %s: %w", module.Name(), err)
		}
	}
	return nil
}
