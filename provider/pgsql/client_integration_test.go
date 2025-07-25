//go:build integration && pgsql
// +build integration,pgsql

package pgsql

import (
	"errors"
	"testing"
)

func TestClientConfigValidate(t *testing.T) {

	defaultCfg := NewClientConfig()
	defaultCfg.DSN = resolveDSN()

	testCases := []struct {
		name     string
		cfg      *ClientConfig
		expected error
	}{
		{
			name:     "Empty Config",
			cfg:      &ClientConfig{},
			expected: ErrEmptyDSN,
		},
		{
			name:     "Default Config",
			cfg:      defaultCfg,
			expected: nil,
		},
		{
			name: "Non-empty DSN",
			cfg: &ClientConfig{
				DSN:          defaultCfg.DSN,
				MaxIdleConns: DefaultIdleConns,
				MaxOpenConns: DefaultMaxConns,
				ConnLifetime: DefaultConnLifeTimeSecond,
				ConnIdleTime: DefaultConnIdleTimeSecond,
			},
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if !errors.Is(err, tc.expected) {
				t.Errorf("expected %v, got %v", tc.expected, err)
			}
			if err == nil {
				_, err := NewClient(tc.cfg)
				if err != nil {
					t.Errorf("expected nil, got %v", err)
				}
			}
		})
	}
}
