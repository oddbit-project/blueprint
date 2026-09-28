package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(c *Config)
		wantErr error
	}{
		{
			name:   "default config",
			modify: func(c *Config) {},
		},
		{
			name:    "empty endpoint",
			modify:  func(c *Config) { c.Endpoint = "" },
			wantErr: ErrMissingEndpoint,
		},
		{
			name:    "tls without cert and key",
			modify:  func(c *Config) { c.TLSEnable = true },
			wantErr: ErrMissingTLSCert,
		},
		{
			name: "tls without key",
			modify: func(c *Config) {
				c.TLSEnable = true
				c.TLSCert = "cert.pem"
			},
			wantErr: ErrMissingTLSCert,
		},
		{
			name: "tls without cert",
			modify: func(c *Config) {
				c.TLSEnable = true
				c.TLSKey = "key.pem"
			},
			wantErr: ErrMissingTLSCert,
		},
		{
			name: "cert and key without tls enabled",
			modify: func(c *Config) {
				c.TLSCert = "cert.pem"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig()
			tt.modify(cfg)
			err := cfg.Validate()
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestConfig_Validate_AppliesDefaults(t *testing.T) {
	cfg := &Config{Endpoint: DefaultEndpoint}
	require.NoError(t, cfg.Validate())
	assert.Equal(t, DefaultPort, cfg.Port)
	assert.Equal(t, DefaultReadTimeout, cfg.ReadTimeout)
	assert.Equal(t, DefaultWriteTimeout, cfg.WriteTimeout)
}

func TestNewServer_EmptyEndpoint(t *testing.T) {
	cfg := NewConfig()
	cfg.Endpoint = ""

	var srv *Server
	var err error
	require.NotPanics(t, func() {
		srv, err = cfg.NewServer()
	})
	assert.ErrorIs(t, err, ErrMissingEndpoint)
	assert.Nil(t, srv)
}
