package config

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Cloak     CloakConfig     `mapstructure:"cloak"`
	Drivers   DriversConfig   `mapstructure:"drivers"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
}

type ServerConfig struct {
	Port            string   `mapstructure:"port"`
	Mode            string   `mapstructure:"mode"` // debug / release
	Timezone        string   `mapstructure:"timezone"`
	ShutdownTimeout int      `mapstructure:"shutdown_timeout"` // 秒
	AllowedOrigins  []string `mapstructure:"allowed_origins"`  // CORS 允许的来源，空则允许全部
}

type CloakConfig struct {
	BaseURL string `mapstructure:"base_url"` // CloakBrowser-Manager 地址
	APIKey  string `mapstructure:"api_key"`  // 可选的 API key
}

type DriversConfig struct {
	DefaultTimeout int      `mapstructure:"default_timeout"` // 秒
	Platforms      []string `mapstructure:"platforms"`       // 启用的平台列表，如 ["115", "pikpak"]
}

type RateLimitConfig struct {
	Enabled bool    `mapstructure:"enabled"`
	RPS     float64 `mapstructure:"rps"`
	Burst   int     `mapstructure:"burst"`
}
