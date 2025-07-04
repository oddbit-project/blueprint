package session

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSessionData(t *testing.T) {
	// Create a session
	s := &SessionData{
		Values: make(map[string]interface{}),
	}

	// Test Set and Get
	s.Set("string", "value")
	val, ok := s.Get("string")
	assert.True(t, ok)
	assert.Equal(t, "value", val)

	// Test GetString
	str, ok := s.GetString("string")
	assert.True(t, ok)
	assert.Equal(t, "value", str)

	// Test Has
	assert.True(t, s.Has("string"))
	assert.False(t, s.Has("nonexistent"))

	// Test Delete
	s.Delete("string")
	assert.False(t, s.Has("string"))

	// Test typed setters and getters
	s.Set("int", 42)
	num, ok := s.GetInt("int")
	assert.True(t, ok)
	assert.Equal(t, 42, num)

	s.Set("bool", true)
	b, ok := s.GetBool("bool")
	assert.True(t, ok)
	assert.Equal(t, true, b)

	// Test wrong type
	s.Set("wrongType", "not an int")
	_, ok = s.GetInt("wrongType")
	assert.False(t, ok)

	// Test Flash messages
	s.Flash("flash value")
	assert.True(t, s.Has("_flash_"))

	flashVal, ok := s.GetFlash()
	assert.True(t, ok)
	assert.Equal(t, "flash value", flashVal)
	assert.False(t, s.Has("_flash_"))

	// Test string flash messages
	s.FlashString("string flash")
	flashStr, ok := s.GetFlashString()
	assert.True(t, ok)
	assert.Equal(t, "string flash", flashStr)

	// Test GetFlashString with wrong type
	s.Flash(123)
	_, ok = s.GetFlashString()
	assert.False(t, ok)
}
