package prometheus

import (
	"github.com/oddbit-project/blueprint/provider/httpserver"
)

const (
	DefaultEndpoint = "/metrics"
	DefaultHost     = "localhost"
	DefaultPort     = 2220
	serverName      = "prometheus"
)

type Config struct {
	Enabled  bool   `json:"enabled"`
	Endpoint string `json:"endpoint"`
	httpserver.ServerConfig
}

func NewConfig() *Config {
	cfg := httpserver.NewServerConfig()
	cfg.Host = DefaultHost
	cfg.Port = DefaultPort
	cfg.ServerName = serverName

	return &Config{
		Enabled:      true,
		Endpoint:     DefaultEndpoint,
		ServerConfig: *cfg,
	}
}

func (c *Config) Validate() error {
	if c.Port == 0 {
		c.Port = DefaultPort
	}
	return c.ServerConfig.Validate()
}
