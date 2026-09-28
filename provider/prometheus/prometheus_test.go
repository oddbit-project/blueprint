package prometheus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfig_Defaults(t *testing.T) {
	cfg := NewConfig()
	assert.True(t, cfg.Enabled)
	assert.Equal(t, DefaultEndpoint, cfg.Endpoint)
	assert.Equal(t, DefaultPort, cfg.Port)
	assert.Equal(t, "localhost", cfg.Host)
}

func TestConfig_Validate_ZeroPortUsesDefaultPort(t *testing.T) {
	cfg := NewConfig()
	cfg.Port = 0
	require.NoError(t, cfg.Validate())
	assert.Equal(t, DefaultPort, cfg.Port)
}

func TestNewServer_Disabled(t *testing.T) {
	cfg := NewConfig()
	cfg.Enabled = false

	srv, err := NewServer(cfg, nil)
	assert.NoError(t, err)
	assert.Nil(t, srv)

	srv, err = cfg.NewServer(nil)
	assert.NoError(t, err)
	assert.Nil(t, srv)
}

func TestNewServer_Enabled(t *testing.T) {
	srv, err := NewServer(NewConfig(), nil)
	require.NoError(t, err)
	require.NotNil(t, srv)
	assert.NotNil(t, srv.Registry())
}
