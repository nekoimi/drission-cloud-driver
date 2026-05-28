package cloak

import "time"

// BrowserProfile represents a browser profile managed by CloakBrowser-Manager.
type BrowserProfile struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Status           string    `json:"status"` // running, stopped, etc.
	CDPUrl           string    `json:"cdp_url,omitempty"`
	Platform         string    `json:"platform,omitempty"`
	Headless         bool      `json:"headless"`
	ScreenWidth      int       `json:"screen_width,omitempty"`
	ScreenHeight     int       `json:"screen_height,omitempty"`
	VNCWSPort        int       `json:"vnc_ws_port,omitempty"`
	UserDataDir      string    `json:"user_data_dir,omitempty"`
	CreatedAt        time.Time `json:"created_at,omitempty"`
	UpdatedAt        time.Time `json:"updated_at,omitempty"`
}
