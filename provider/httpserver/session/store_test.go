package session

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestMemoryStore(t *testing.T) {
	config := DefaultSessionConfig()
	store := NewMemoryStore(config)
	
	// Test generate
	session, id := store.Generate()
	assert.NotEmpty(t, id)
	assert.NotNil(t, session)
	assert.Empty(t, session.Values)
	
	// Test set
	session.Values["test"] = "value"
	err := store.Set(id, session)
	assert.NoError(t, err)
	
	// Test get
	retrievedSession, err := store.Get(id)
	assert.NoError(t, err)
	assert.Equal(t, "value", retrievedSession.Values["test"])
	
	// Test delete
	err = store.Delete(id)
	assert.NoError(t, err)
	
	// Test get after delete
	_, err = store.Get(id)
	assert.Error(t, err)
	assert.Equal(t, ErrSessionNotFound, err)
}

func TestMemoryStoreExpiration(t *testing.T) {
	config := DefaultSessionConfig()
	// Set a very short expiration for testing
	config.Expiration = 50 * time.Millisecond
	store := NewMemoryStore(config)
	
	// Create a session
	session, id := store.Generate()
	err := store.Set(id, session)
	assert.NoError(t, err)
	
	// Verify it exists
	_, err = store.Get(id)
	assert.NoError(t, err)
	
	// Wait for it to expire
	time.Sleep(100 * time.Millisecond)
	
	// Verify it's expired
	_, err = store.Get(id)
	assert.Error(t, err)
	assert.Equal(t, ErrSessionExpired, err)
}

func TestMemoryStoreIdleTimeout(t *testing.T) {
	config := DefaultSessionConfig()
	// Set a very short idle timeout for testing
	config.IdleTimeout = 50 * time.Millisecond
	store := NewMemoryStore(config)
	
	// Create a session
	session, id := store.Generate()
	err := store.Set(id, session)
	assert.NoError(t, err)
	
	// Verify it exists
	_, err = store.Get(id)
	assert.NoError(t, err)
	
	// Wait for it to exceed idle timeout
	time.Sleep(100 * time.Millisecond)
	
	// Verify it's expired due to idle timeout
	_, err = store.Get(id)
	assert.Error(t, err)
	assert.Equal(t, ErrSessionExpired, err)
}

func TestMemoryStoreMaxSessions(t *testing.T) {
	config := DefaultSessionConfig()
	// Set max sessions to a small number for testing
	config.MaxSessions = 2
	store := NewMemoryStore(config)
	
	// Create 3 sessions, which should cause the oldest to be removed
	session1, id1 := store.Generate()
	err := store.Set(id1, session1)
	assert.NoError(t, err)
	
	// Add a small delay to ensure different last accessed times
	time.Sleep(10 * time.Millisecond)
	
	session2, id2 := store.Generate()
	err = store.Set(id2, session2)
	assert.NoError(t, err)
	
	time.Sleep(10 * time.Millisecond)
	
	session3, id3 := store.Generate()
	err = store.Set(id3, session3)
	assert.NoError(t, err)
	
	// The first session should have been removed to make room for the third
	_, err = store.Get(id1)
	assert.Error(t, err)
	assert.Equal(t, ErrSessionNotFound, err)
	
	// The second and third sessions should still exist
	_, err = store.Get(id2)
	assert.NoError(t, err)
	
	_, err = store.Get(id3)
	assert.NoError(t, err)
}