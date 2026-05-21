package browser

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"go.uber.org/zap"
)

// Connection wraps a rod browser connection to a remote browser via CDP.
type Connection struct {
	profileID string
	cdpURL    string
	browser   *rod.Browser
	logger    *zap.Logger
}

// NewConnection creates a new CDP connection using go-rod.
func NewConnection(ctx context.Context, profileID, cdpURL string, logger *zap.Logger) (*Connection, error) {
	logger.Info("connecting to browser", zap.String("profile", profileID), zap.String("cdp", cdpURL))

	browser := rod.New().ControlURL(cdpURL)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("connect to browser: %w", err)
	}

	return &Connection{
		profileID: profileID,
		cdpURL:    cdpURL,
		browser:   browser,
		logger:    logger,
	}, nil
}

// Navigate navigates to a URL.
func (c *Connection) Navigate(ctx context.Context, url string) error {
	page, err := c.browser.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		return fmt.Errorf("navigate to %s: %w", url, err)
	}
	return page.WaitLoad()
}

// Evaluate executes JavaScript expression and returns the result.
func (c *Connection) Evaluate(ctx context.Context, expression string) (json.RawMessage, error) {
	page, err := c.browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, fmt.Errorf("get page: %w", err)
	}

	result, err := page.Eval(expression)
	if err != nil {
		return nil, fmt.Errorf("evaluate: %w", err)
	}

	return json.RawMessage(result.Value.Str()), nil
}

// Close closes the browser connection.
func (c *Connection) Close() error {
	c.logger.Info("closing browser connection", zap.String("profile", c.profileID))
	return c.browser.Close()
}

// Browser returns the underlying rod browser instance.
func (c *Connection) Browser() *rod.Browser {
	return c.browser
}
