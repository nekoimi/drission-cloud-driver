package browser

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/cloak"
)

// Manager manages browser connections for multiple profiles.
type Manager struct {
	cloak  *cloak.Client
	conns  map[string]*Connection
	mu     sync.RWMutex
	logger *zap.Logger
}

// NewManager creates a new browser manager.
func NewManager(cloakClient *cloak.Client, logger *zap.Logger) *Manager {
	return &Manager{
		cloak:  cloakClient,
		conns:  make(map[string]*Connection),
		logger: logger,
	}
}

// GetConnection returns an existing connection or creates a new one for the given profile.
func (m *Manager) GetConnection(ctx context.Context, profileID string) (*Connection, error) {
	m.mu.RLock()
	conn, ok := m.conns[profileID]
	m.mu.RUnlock()

	if ok {
		return conn, nil
	}

	// Create new connection
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double check after acquiring write lock
	if conn, ok := m.conns[profileID]; ok {
		return conn, nil
	}

	cdpURL, err := m.cloak.GetCDPEndpoint(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("get CDP endpoint for profile %s: %w", profileID, err)
	}

	conn, err = NewConnection(ctx, profileID, cdpURL, m.logger)
	if err != nil {
		return nil, fmt.Errorf("create connection for profile %s: %w", profileID, err)
	}

	m.conns[profileID] = conn
	return conn, nil
}

// CloseConnection closes and removes a connection for the given profile.
func (m *Manager) CloseConnection(profileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, ok := m.conns[profileID]
	if !ok {
		return nil
	}

	delete(m.conns, profileID)
	return conn.Close()
}

// ListProfiles returns all browser profiles from CloakBrowser-Manager.
func (m *Manager) ListProfiles(ctx context.Context) ([]cloak.BrowserProfile, error) {
	return m.cloak.ListProfiles(ctx)
}

// GetProfile returns a browser profile from CloakBrowser-Manager.
func (m *Manager) GetProfile(ctx context.Context, profileID string) (*cloak.BrowserProfile, error) {
	return m.cloak.GetProfile(ctx, profileID)
}

// StartProfile starts a browser profile in CloakBrowser-Manager.
func (m *Manager) StartProfile(ctx context.Context, profileID string) error {
	return m.cloak.StartProfile(ctx, profileID)
}

// StopProfile stops a browser profile and clears any cached CDP connection.
func (m *Manager) StopProfile(ctx context.Context, profileID string) error {
	if err := m.CloseConnection(profileID); err != nil {
		return err
	}
	return m.cloak.StopProfile(ctx, profileID)
}

// Shutdown closes all connections.
func (m *Manager) Shutdown() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for id, conn := range m.conns {
		if err := conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close %s: %w", id, err))
		}
	}

	m.conns = make(map[string]*Connection)

	if len(errs) > 0 {
		return fmt.Errorf("close connections: %v", errs)
	}
	return nil
}
