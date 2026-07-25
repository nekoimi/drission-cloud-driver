package drivers

import (
	"fmt"
	"sync"

	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/browser"
)

// Factory creates a Driver instance.
type Factory func(browserMgr *browser.Manager, logger *zap.Logger) (Driver, error)

// Registry manages driver factories and instances.
type Registry struct {
	mu        sync.Mutex
	factories map[string]Factory
	drivers   map[string]Driver
	logger    *zap.Logger
}

// NewRegistry creates a new driver registry.
func NewRegistry(logger *zap.Logger) *Registry {
	return &Registry{
		factories: make(map[string]Factory),
		drivers:   make(map[string]Driver),
		logger:    logger,
	}
}

// Register registers a driver factory for a platform.
func (r *Registry) Register(platform string, factory Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[platform] = factory
	r.logger.Info("registered driver", zap.String("platform", platform))
}

// Get returns a driver for the given platform, creating it if necessary.
func (r *Registry) Get(platform string, browserMgr *browser.Manager) (Driver, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if driver, ok := r.drivers[platform]; ok {
		return driver, nil
	}

	factory, ok := r.factories[platform]
	if !ok {
		return nil, fmt.Errorf("driver not found for platform: %s", platform)
	}

	driver, err := factory(browserMgr, r.logger)
	if err != nil {
		return nil, fmt.Errorf("create driver for %s: %w", platform, err)
	}

	r.drivers[platform] = driver
	return driver, nil
}

// ListPlatforms returns all registered platform names.
func (r *Registry) ListPlatforms() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	platforms := make([]string, 0, len(r.factories))
	for p := range r.factories {
		platforms = append(platforms, p)
	}
	return platforms
}

// Close releases resources held by instantiated drivers.
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for platform, driver := range r.drivers {
		closer, ok := driver.(interface{ Close() error })
		if !ok {
			continue
		}
		if err := closer.Close(); err != nil {
			return fmt.Errorf("close driver %s: %w", platform, err)
		}
	}
	return nil
}
