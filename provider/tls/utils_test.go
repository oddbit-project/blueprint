package tls

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTlsKeyCredential_FetchEnvVarRepeatable(t *testing.T) {
	const envVar = "TEST_TLS_REFETCH_PASSWORD"
	t.Setenv(envVar, "s3cret")
	cred := TlsKeyCredential{PasswordEnvVar: envVar}

	first, err := cred.Fetch()
	require.NoError(t, err)
	assert.Equal(t, "s3cret", first)

	second, err := cred.Fetch()
	require.NoError(t, err)
	assert.Equal(t, "s3cret", second)
	assert.Equal(t, "s3cret", os.Getenv(envVar))
}
