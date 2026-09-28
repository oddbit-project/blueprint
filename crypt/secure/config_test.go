package secure

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetch_EnvVarRepeatable(t *testing.T) {
	tests := []struct {
		name   string
		envVar string
		fetch  func(envVar string) (string, error)
	}{
		{
			name:   "DefaultCredentialConfig",
			envVar: "TEST_SECURE_REFETCH_PASSWORD",
			fetch: func(envVar string) (string, error) {
				return DefaultCredentialConfig{PasswordEnvVar: envVar}.Fetch()
			},
		},
		{
			name:   "KeyConfig",
			envVar: "TEST_SECURE_REFETCH_KEY",
			fetch: func(envVar string) (string, error) {
				return KeyConfig{KeyEnvVar: envVar}.Fetch()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envVar, "s3cret")

			first, err := tt.fetch(tt.envVar)
			require.NoError(t, err)
			assert.Equal(t, "s3cret", first)

			second, err := tt.fetch(tt.envVar)
			require.NoError(t, err)
			assert.Equal(t, "s3cret", second)
			assert.Equal(t, "s3cret", os.Getenv(tt.envVar))
		})
	}
}
