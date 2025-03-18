package session

import (
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestHelpers(t *testing.T) {
	// Create a test context
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	
	// Create a session
	session := &SessionData{
		Values: make(map[string]interface{}),
	}
	
	// Set session in context
	c.Set(ContextSessionKey, session)
	
	// Test Set and Get
	Set(c, "string", "value")
	val, ok := GetValue(c, "string")
	assert.True(t, ok)
	assert.Equal(t, "value", val)
	
	// Test GetString
	str, ok := GetString(c, "string")
	assert.True(t, ok)
	assert.Equal(t, "value", str)
	
	// Test Has
	assert.True(t, Has(c, "string"))
	assert.False(t, Has(c, "nonexistent"))
	
	// Test Delete
	Delete(c, "string")
	assert.False(t, Has(c, "string"))
	
	// Test typed setters and getters
	Set(c, "int", 42)
	num, ok := GetInt(c, "int")
	assert.True(t, ok)
	assert.Equal(t, 42, num)
	
	Set(c, "bool", true)
	b, ok := GetBool(c, "bool")
	assert.True(t, ok)
	assert.Equal(t, true, b)
	
	// Test wrong type
	Set(c, "wrongType", "not an int")
	_, ok = GetInt(c, "wrongType")
	assert.False(t, ok)
	
	// Test Flash messages
	Flash(c, "flash", "flash value")
	assert.True(t, Has(c, "_flash_flash"))
	
	flashVal, ok := GetFlash(c, "flash")
	assert.True(t, ok)
	assert.Equal(t, "flash value", flashVal)
	assert.False(t, Has(c, "_flash_flash"))
	
	// Test string flash messages
	FlashString(c, "strFlash", "string flash")
	flashStr, ok := GetFlashString(c, "strFlash")
	assert.True(t, ok)
	assert.Equal(t, "string flash", flashStr)
	
	// Test GetFlashString with wrong type
	Flash(c, "numFlash", 123)
	_, ok = GetFlashString(c, "numFlash")
	assert.False(t, ok)
}