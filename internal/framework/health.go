package framework

import (
	"context"
	"sync"
)

type HealthCheck func(ctx context.Context) error

type HealthRegistry struct {
	mu     sync.RWMutex
	checks map[string]HealthCheck
}

type HealthResult struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

func NewHealthRegistry() *HealthRegistry {
	return &HealthRegistry{
		checks: make(map[string]HealthCheck),
	}
}

func (r *HealthRegistry) Register(name string, check HealthCheck) {
	if r == nil || name == "" || check == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checks[name] = check
}

func (r *HealthRegistry) Check(ctx context.Context) HealthResult {
	result := HealthResult{
		Status: "ready",
		Checks: make(map[string]string),
	}
	if r == nil {
		return result
	}

	r.mu.RLock()
	checks := make(map[string]HealthCheck, len(r.checks))
	for name, check := range r.checks {
		checks[name] = check
	}
	r.mu.RUnlock()

	for name, check := range checks {
		if err := check(ctx); err != nil {
			result.Status = "not ready"
			result.Checks[name] = err.Error()
			continue
		}
		result.Checks[name] = "ok"
	}
	return result
}
