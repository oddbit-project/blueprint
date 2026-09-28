package session

import (
	"testing"
	"time"

	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/provider/kv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore_ValidConfig(t *testing.T) {
	config := NewConfig()
	store, err := NewStore(config, kv.NewMemoryKV(), nil)

	assert.NoError(t, err)
	assert.NotNil(t, store)
}

func TestNewStore_InvalidExpirationSeconds(t *testing.T) {
	config := NewConfig()
	config.ExpirationSeconds = 0

	store, err := NewStore(config, kv.NewMemoryKV(), nil)

	assert.ErrorIs(t, err, ErrInvalidExpirationSeconds)
	assert.Nil(t, store)
}

func TestNewStore_InvalidIdleTimeoutSeconds(t *testing.T) {
	config := NewConfig()
	config.IdleTimeoutSeconds = -1

	store, err := NewStore(config, kv.NewMemoryKV(), nil)

	assert.ErrorIs(t, err, ErrInvalidIdleTimeoutSeconds)
	assert.Nil(t, store)
}

func TestNewStore_InvalidCleanupIntervalSeconds(t *testing.T) {
	config := NewConfig()
	config.CleanupIntervalSeconds = 0

	store, err := NewStore(config, kv.NewMemoryKV(), nil)

	assert.ErrorIs(t, err, ErrInvalidCleanupIntervalSeconds)
	assert.Nil(t, store)
}

func TestNewStore_InvalidSameSite(t *testing.T) {
	config := NewConfig()
	config.SameSite = 99

	store, err := NewStore(config, kv.NewMemoryKV(), nil)

	assert.ErrorIs(t, err, ErrInvalidSameSite)
	assert.Nil(t, store)
}

func TestNewStore_NilConfigUsesDefaults(t *testing.T) {
	store, err := NewStore(nil, kv.NewMemoryKV(), nil)

	assert.NoError(t, err)
	assert.NotNil(t, store)
}

func newEncryptedTestStore(t *testing.T, backend kv.KV, key string) *Store {
	t.Helper()
	config := NewConfig()
	config.EncryptionKey = secure.DefaultCredentialConfig{Password: key}
	store, err := NewStore(config, backend, nil)
	require.NoError(t, err)
	return store
}

// Issue #87: Store.Get must surface the decryption error of an encrypted session
func TestStore_Get_Issue87_DecryptionErrorIsReturned(t *testing.T) {
	const key = "0123456789abcdef0123456789abcdef"

	tests := []struct {
		name    string
		readKey string
		alter   func([]byte) []byte
		wantErr error
	}{
		{
			name:    "tampered data",
			readKey: key,
			alter: func(data []byte) []byte {
				tampered := append([]byte{}, data...)
				tampered[len(tampered)-1] ^= 0xff
				return tampered
			},
			wantErr: secure.ErrAuthenticationFailed,
		},
		{
			name:    "different key",
			readKey: "fedcba9876543210fedcba9876543210",
			wantErr: secure.ErrAuthenticationFailed,
		},
		{
			name:    "truncated data",
			readKey: key,
			alter: func(data []byte) []byte {
				return data[:10]
			},
			wantErr: secure.ErrDataTooShort,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := kv.NewMemoryKV()
			writer := newEncryptedTestStore(t, backend, key)
			reader := newEncryptedTestStore(t, backend, tt.readKey)

			session, id := writer.Generate()
			require.NoError(t, writer.Set(id, session))

			if tt.alter != nil {
				data, err := backend.Get(id)
				require.NoError(t, err)
				require.NoError(t, backend.SetTTL(id, tt.alter(data), time.Minute))
			}

			got, err := reader.Get(id)
			assert.Nil(t, got)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

// Issue #87 guard: an untampered encrypted session is stored encrypted and still returned
func TestStore_Get_Issue87_EncryptedSessionRoundTrip(t *testing.T) {
	backend := kv.NewMemoryKV()
	store := newEncryptedTestStore(t, backend, "0123456789abcdef0123456789abcdef")

	session, id := store.Generate()
	session.Set("user", "alice-plaintext-marker")
	require.NoError(t, store.Set(id, session))

	stored, err := backend.Get(id)
	require.NoError(t, err)
	assert.NotContains(t, string(stored), "alice-plaintext-marker")

	got, err := store.Get(id)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "alice-plaintext-marker", got.Values["user"])
}
