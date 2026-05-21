package cloak

// BrowserProfile represents a browser profile managed by CloakBrowser-Manager.
type BrowserProfile struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"` // running, stopped, etc.
	CDPUrl string `json:"cdp_url,omitempty"`
}

// ListProfilesResponse is the response from listing profiles.
type ListProfilesResponse struct {
	Profiles []BrowserProfile `json:"profiles"`
}

// ProfileResponse is the response from getting a single profile.
type ProfileResponse struct {
	Profile BrowserProfile `json:"profile"`
}
