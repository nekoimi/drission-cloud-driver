package cloak

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/config"
)

// Client is an HTTP client for CloakBrowser-Manager API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewClient creates a new CloakBrowser-Manager client.
func NewClient(cfg config.CloakConfig, logger *zap.Logger) *Client {
	return &Client{
		baseURL: cfg.BaseURL,
		apiKey:  cfg.APIKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// ListProfiles returns all browser profiles.
func (c *Client) ListProfiles(ctx context.Context) ([]BrowserProfile, error) {
	var resp ListProfilesResponse
	if err := c.do(ctx, http.MethodGet, "/api/profiles", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Profiles, nil
}

// GetProfile returns a specific browser profile.
func (c *Client) GetProfile(ctx context.Context, profileID string) (*BrowserProfile, error) {
	var resp ProfileResponse
	path := fmt.Sprintf("/api/profiles/%s", profileID)
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Profile, nil
}

// StartProfile starts a browser profile.
func (c *Client) StartProfile(ctx context.Context, profileID string) error {
	path := fmt.Sprintf("/api/profiles/%s/start", profileID)
	return c.do(ctx, http.MethodPost, path, nil, nil)
}

// StopProfile stops a browser profile.
func (c *Client) StopProfile(ctx context.Context, profileID string) error {
	path := fmt.Sprintf("/api/profiles/%s/stop", profileID)
	return c.do(ctx, http.MethodPost, path, nil, nil)
}

// GetCDPEndpoint returns the CDP WebSocket endpoint for a profile.
func (c *Client) GetCDPEndpoint(ctx context.Context, profileID string) (string, error) {
	profile, err := c.GetProfile(ctx, profileID)
	if err != nil {
		return "", err
	}
	if profile.CDPUrl == "" {
		return "", fmt.Errorf("profile %s has no CDP endpoint", profileID)
	}
	return profile.CDPUrl, nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, result any) error {
	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	c.logger.Debug("cloak request", zap.String("method", method), zap.String("url", url))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("cloak API error: status %d", resp.StatusCode)
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}
