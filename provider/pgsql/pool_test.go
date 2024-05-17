package pgsql

import (
	"context"
	"testing"
)

func TestPoolConfigValidate(t *testing.T) {

	defaultCfg := NewPoolConfig()
	defaultCfg.DSN = getDSN()

	testCases := []struct {
		name     string
		cfg      *PoolConfig
		expected error
	}{
		{
			name:     "Empty Config",
			cfg:      &PoolConfig{},
			expected: ErrEmptyDSN,
		},
		{
			name:     "Default Config",
			cfg:      defaultCfg,
			expected: nil,
		},
		{
			name: "Non-empty DSN",
			cfg: &PoolConfig{
				DSN:                 defaultCfg.DSN,
				MinConns:            DefaultMinConns,
				MaxConns:            DefaultMaxConns,
				ConnLifeTime:        DefaultConnLifeTimeSecond,
				ConnIdleTime:        DefaultConnIdleTimeSecond,
				HealthCheckInterval: DefaultHealthCheckSecond,
				ConnTimeout:         DefaultConnTimeoutSecond,
				BeforeConnect:       nil,
				AfterConnect:        nil,
				BeforeAcquire:       nil,
				AfterRelease:        nil,
				BeforeClose:         nil,
			},
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if err != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, err)
			}
			if err == nil {
				_, err := NewPool(context.Background(), tc.cfg)
				if err != nil {
					t.Errorf("expected nil, got %v", err)
				}
			}
		})
	}
}
