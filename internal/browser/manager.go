package browser

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/cloak"
)

// Manager manages browser connections for multiple profiles.
type Manager struct {
	cloak        *cloak.Client
	conns        map[string]*Connection
	profileLocks map[string]*sync.Mutex
	mu           sync.RWMutex
	logger       *zap.Logger
}

const (
	cdpReadyTimeout = 30 * time.Second
	cdpPollInterval = 500 * time.Millisecond
	profileRunning  = "running"
)

// NewManager creates a new browser manager.
func NewManager(cloakClient *cloak.Client, logger *zap.Logger) *Manager {
	return &Manager{
		cloak:        cloakClient,
		conns:        make(map[string]*Connection),
		profileLocks: make(map[string]*sync.Mutex),
		logger:       logger,
	}
}

type connectionCleanup func() error

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

// GetCookieString starts the profile on demand, reads cookies, and releases
// browser resources if this call launched the profile.
func (m *Manager) GetCookieString(ctx context.Context, profileID, domain string) (string, error) {
	return m.GetCookieStringWithWakeURL(ctx, profileID, domain, "")
}

// GetCookieStringWithWakeURL reads cookies and navigates to wakeURL once when
// the initial cookie read is empty.
func (m *Manager) GetCookieStringWithWakeURL(ctx context.Context, profileID, domain, wakeURL string) (string, error) {
	return m.GetCookieStringFor(ctx, profileID, CookieRequest{
		Domain:  domain,
		WakeURL: wakeURL,
	})
}

// GetCookieStringFor reads cookies for a platform. If WakeURL is set and the
// first read is empty, it opens that URL once and retries.
func (m *Manager) GetCookieStringFor(ctx context.Context, profileID string, req CookieRequest) (string, error) {
	unlock := m.lockProfile(profileID)
	defer unlock()

	conn, cleanup, err := m.openTransientConnection(ctx, profileID)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := cleanup(); err != nil {
			m.logger.Warn("failed to cleanup transient browser connection",
				zap.String("profile", profileID),
				zap.Error(err),
			)
		}
	}()

	cookieStr, err := conn.GetCookieString(ctx, req.Domain)
	if err != nil || cookieStr != "" || strings.TrimSpace(req.WakeURL) == "" {
		return cookieStr, err
	}

	m.logger.Info("cookie not found, opening platform URL before retry",
		zap.String("profile", profileID),
		zap.String("domain", req.Domain),
		zap.String("url", req.WakeURL),
	)
	if err := conn.Navigate(ctx, req.WakeURL); err != nil {
		return "", err
	}

	return conn.GetCookieString(ctx, req.Domain)
}

func (m *Manager) openTransientConnection(ctx context.Context, profileID string) (*Connection, connectionCleanup, error) {
	profile, err := m.GetProfile(ctx, profileID)
	if err != nil {
		return nil, nil, fmt.Errorf("get profile %s: %w", profileID, err)
	}

	startedByCall := !strings.EqualFold(profile.Status, profileRunning)
	if startedByCall {
		m.logger.Info("starting browser profile for cookie read", zap.String("profile", profileID))
		if _, err := m.cloak.LaunchProfile(ctx, profileID); err != nil {
			return nil, nil, fmt.Errorf("start profile %s: %w", profileID, err)
		}
	}

	cdpURL, err := m.waitForCDPEndpoint(ctx, profileID)
	if err != nil {
		if startedByCall {
			return nil, nil, errors.Join(err, m.stopProfileWithBackgroundContext(profileID))
		}
		return nil, nil, err
	}

	conn, err := NewConnection(ctx, profileID, cdpURL, m.logger)
	if err != nil {
		if startedByCall {
			return nil, nil, errors.Join(
				fmt.Errorf("create connection for profile %s: %w", profileID, err),
				m.stopProfileWithBackgroundContext(profileID),
			)
		}
		return nil, nil, fmt.Errorf("create connection for profile %s: %w", profileID, err)
	}

	cleanup := func() error {
		var errs []error
		if err := conn.Close(); err != nil {
			errs = append(errs, err)
		}
		if startedByCall {
			m.logger.Info("stopping browser profile after cookie read", zap.String("profile", profileID))
			if err := m.stopProfileWithBackgroundContext(profileID); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}

	return conn, cleanup, nil
}

func (m *Manager) waitForCDPEndpoint(ctx context.Context, profileID string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, cdpReadyTimeout)
	defer cancel()

	var lastErr error
	for {
		cdpURL, err := m.cloak.GetCDPEndpoint(ctx, profileID)
		if err == nil {
			return cdpURL, nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return "", fmt.Errorf("wait for CDP endpoint for profile %s: %w: %w", profileID, ctx.Err(), lastErr)
		case <-time.After(cdpPollInterval):
		}
	}
}

func (m *Manager) stopProfileWithBackgroundContext(profileID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return m.StopProfile(ctx, profileID)
}

func (m *Manager) lockProfile(profileID string) func() {
	m.mu.Lock()
	lock, ok := m.profileLocks[profileID]
	if !ok {
		lock = &sync.Mutex{}
		m.profileLocks[profileID] = lock
	}
	m.mu.Unlock()

	lock.Lock()
	return lock.Unlock
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
