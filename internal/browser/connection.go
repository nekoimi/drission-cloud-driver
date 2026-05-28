package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/playwright-community/playwright-go"
	"go.uber.org/zap"
)

// Connection wraps a playwright browser connection to a remote browser via CDP.
type Connection struct {
	profileID string
	cdpURL    string
	pw        *playwright.Playwright
	browser   playwright.Browser
	logger    *zap.Logger
}

// NewConnection creates a new CDP connection using playwright-go.
func NewConnection(ctx context.Context, profileID, cdpURL string, logger *zap.Logger) (*Connection, error) {
	logger.Info("connecting to browser", zap.String("profile", profileID), zap.String("cdp", cdpURL))

	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("start playwright: %w", err)
	}

	browser, err := pw.Chromium.ConnectOverCDP(cdpURL)
	if err != nil {
		pw.Stop()
		return nil, fmt.Errorf("connect to browser: %w", err)
	}

	return &Connection{
		profileID: profileID,
		cdpURL:    cdpURL,
		pw:        pw,
		browser:   browser,
		logger:    logger,
	}, nil
}

// Navigate navigates to a URL.
func (c *Connection) Navigate(ctx context.Context, url string) error {
	contexts := c.browser.Contexts()
	if len(contexts) == 0 {
		return fmt.Errorf("no browser context available")
	}

	page, err := contexts[0].NewPage()
	if err != nil {
		return fmt.Errorf("create page: %w", err)
	}

	_, err = page.Goto(url)
	if err != nil {
		return fmt.Errorf("navigate to %s: %w", url, err)
	}

	return nil
}

// Evaluate executes JavaScript expression and returns the result.
func (c *Connection) Evaluate(ctx context.Context, expression string) (json.RawMessage, error) {
	contexts := c.browser.Contexts()
	if len(contexts) == 0 {
		return nil, fmt.Errorf("no browser context available")
	}

	pages := contexts[0].Pages()
	if len(pages) == 0 {
		return nil, fmt.Errorf("no page available")
	}

	result, err := pages[0].Evaluate(expression, nil)
	if err != nil {
		return nil, fmt.Errorf("evaluate: %w", err)
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}

	return json.RawMessage(jsonBytes), nil
}

// Close closes the browser connection.
func (c *Connection) Close() error {
	c.logger.Info("closing browser connection", zap.String("profile", c.profileID))

	if err := c.browser.Close(); err != nil {
		c.logger.Warn("failed to close browser", zap.Error(err))
	}

	return c.pw.Stop()
}

// Browser returns the underlying playwright browser instance.
func (c *Connection) Browser() playwright.Browser {
	return c.browser
}

// GetCookies returns all cookies for the given domain.
func (c *Connection) GetCookies(ctx context.Context, domain string) ([]playwright.Cookie, error) {
	contexts := c.browser.Contexts()
	if len(contexts) == 0 {
		return nil, fmt.Errorf("no browser context available")
	}

	cookies, err := contexts[0].Cookies()
	if err != nil {
		return nil, fmt.Errorf("get cookies: %w", err)
	}

	// Filter by domain if specified
	if domain == "" {
		return cookies, nil
	}

	var filtered []playwright.Cookie
	for _, cookie := range cookies {
		if cookie.Domain == domain || cookie.Domain == "."+domain {
			filtered = append(filtered, cookie)
		}
	}

	return filtered, nil
}

// GetCookieString returns cookies as a string for the given domain.
func (c *Connection) GetCookieString(ctx context.Context, domain string) (string, error) {
	cookies, err := c.GetCookies(ctx, domain)
	if err != nil {
		return "", err
	}

	var parts []string
	for _, cookie := range cookies {
		parts = append(parts, cookie.Name+"="+cookie.Value)
	}

	return strings.Join(parts, ";"), nil
}
