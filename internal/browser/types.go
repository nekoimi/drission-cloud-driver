package browser

// ConnectionInfo holds information about a browser connection.
type ConnectionInfo struct {
	ProfileID string
	CDPUrl    string
}

// CookieRequest describes how to read platform cookies from a browser profile.
type CookieRequest struct {
	Domain  string
	WakeURL string
}
