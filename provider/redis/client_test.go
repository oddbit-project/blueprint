package redis

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClientTLS(t *testing.T) {
	tests := []struct {
		name      string
		tlsEnable bool
	}{
		{"TLS disabled", false},
		{"TLS enabled", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.TLSEnable = tt.tlsEnable

			client, err := NewClient(cfg)
			require.NoError(t, err)
			defer func() { _ = client.Close() }()

			if tt.tlsEnable {
				assert.NotNil(t, client.Redis.Options().TLSConfig, "TLSEnable must configure TLS on the redis client")
			} else {
				assert.Nil(t, client.Redis.Options().TLSConfig)
			}
		})
	}
}
