package base

import (
	"context"

	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/browser"
	"github.com/nekoimi/drission-cloud-driver/internal/drivers"
)

// Base provides common functionality for all drivers.
type Base struct {
	Platform_     string
	Capabilities_ drivers.DriverCapabilities
	BrowserMgr    *browser.Manager
	Logger        *zap.Logger
}

// Platform returns the platform identifier.
func (b *Base) Platform() string {
	return b.Platform_
}

// Capabilities returns driver capabilities.
func (b *Base) Capabilities() drivers.DriverCapabilities {
	return b.Capabilities_
}

// GetConn returns a browser connection for the given profile.
func (b *Base) GetConn(ctx context.Context, profileID string) (*browser.Connection, error) {
	return b.BrowserMgr.GetConnection(ctx, profileID)
}
