package prometheus

import (
	"github.com/oddbit-project/blueprint/provider/httpserver"
)

const (
	DefaultEndpoint = "/metrics"
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
	cfg.Port = DefaultPort
	cfg.Options["serverName"] = serverName

	return &Config{
		Enabled:      true,
		Endpoint:     DefaultEndpoint,
		ServerConfig: *cfg,
	}
}

func (c *Config) Validate() error {
	return c.ServerConfig.Validate()
}
