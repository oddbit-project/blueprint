package pgsql

import (
	"testing"
)

func TestClientConfigValidate(t *testing.T) {
	testCases := []struct {
		name     string
		dsn      string
		expected error
	}{
		{
			name:     "Empty DSN",
			dsn:      "",
			expected: ErrEmptyDSN,
		},
		{
			name:     "Non-empty DSN",
			dsn:      "postgresql://blueprint:password@postgres:5432/blueprint",
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &ClientConfig{
				DSN: tc.dsn,
			}
			err := config.Validate()

			if err != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, err)
			}
		})
	}
}
