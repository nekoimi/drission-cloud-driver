package module

import (
	"sort"
	"sync"

	"github.com/nekoimi/drission-cloud-driver/internal/framework"
)

type Scope string

const (
	ScopeHTTP      Scope = "http"
	ScopeScheduler Scope = "scheduler"
)

type Entry struct {
	Module framework.Module
	Scopes []Scope
}

var (
	mu      sync.RWMutex
	entries []Entry
)

func Register(mod framework.Module, scopes ...Scope) {
	if mod == nil {
		return
	}
	if len(scopes) == 0 {
		scopes = []Scope{ScopeHTTP}
	}

	mu.Lock()
	defer mu.Unlock()
	entries = append(entries, Entry{
		Module: mod,
		Scopes: append([]Scope(nil), scopes...),
	})
}

func Modules(scopes ...Scope) []framework.Module {
	if len(scopes) == 0 {
		scopes = []Scope{ScopeHTTP}
	}

	mu.RLock()
	defer mu.RUnlock()

	modules := make([]framework.Module, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.hasAnyScope(scopes...) {
			if _, ok := seen[entry.Module.Name()]; ok {
				continue
			}
			seen[entry.Module.Name()] = struct{}{}
			modules = append(modules, entry.Module)
		}
	}
	sort.SliceStable(modules, func(i, j int) bool {
		return modules[i].Name() < modules[j].Name()
	})
	return modules
}

func (e Entry) hasAnyScope(scopes ...Scope) bool {
	for _, s := range e.Scopes {
		for _, scope := range scopes {
			if s == scope {
				return true
			}
		}
	}
	return false
}
