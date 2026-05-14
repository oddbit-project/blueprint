package httpserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServerConfig_Validate_DefaultServerName(t *testing.T) {
	cfg := &ServerConfig{}
	err := cfg.Validate()

	assert.NoError(t, err)
	assert.Equal(t, ServerDefaultName, cfg.ServerName)
}

func TestServerConfig_Validate_DefaultPort(t *testing.T) {
	cfg := &ServerConfig{}
	err := cfg.Validate()

	assert.NoError(t, err)
	assert.Equal(t, ServerDefaultPort, cfg.Port)
}

func TestServerConfig_Validate_NegativePort(t *testing.T) {
	cfg := &ServerConfig{Port: -1}
	err := cfg.Validate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "port must be between 0 and 65535")
}

func TestServerConfig_Validate_PortTooHigh(t *testing.T) {
	cfg := &ServerConfig{Port: 70000}
	err := cfg.Validate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "port must be between 0 and 65535")
}

func TestServerConfig_Validate_ValidPort(t *testing.T) {
	cfg := &ServerConfig{Port: 8080}
	err := cfg.Validate()

	assert.NoError(t, err)
	assert.Equal(t, 8080, cfg.Port)
}
