package session

import (
	"testing"

	"github.com/oddbit-project/blueprint/provider/kv"
	"github.com/stretchr/testify/assert"
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
