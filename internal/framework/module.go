package framework

import "fmt"

type Module interface {
	Name() string
	Register(ctx *ModuleContext) error
}

func RegisterModules(ctx *ModuleContext, modules ...Module) error {
	for _, module := range modules {
		if module == nil {
			continue
		}
		if err := module.Register(ctx); err != nil {
			return fmt.Errorf("register module %s: %w", module.Name(), err)
		}
	}
	return nil
}
