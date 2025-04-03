package session

import (
	"github.com/gin-gonic/gin"
)

// Set stores a value in the session
func Set(c *gin.Context, key string, value interface{}) {
	session := Get(c)
	if session != nil {
		session.Values[key] = value
		c.Set(ContextSessionKey, session)
	}
}

// GetValue retrieves a value from the session
func GetValue(c *gin.Context, key string) (interface{}, bool) {
	session := Get(c)
	if session != nil {
		val, exists := session.Values[key]
		return val, exists
	}
	return nil, false
}

// GetString retrieves a string value from the session
func GetString(c *gin.Context, key string) (string, bool) {
	val, exists := GetValue(c, key)
	if !exists {
		return "", false
	}
	
	str, ok := val.(string)
	return str, ok
}

// GetInt retrieves an int value from the session
func GetInt(c *gin.Context, key string) (int, bool) {
	val, exists := GetValue(c, key)
	if !exists {
		return 0, false
	}
	
	num, ok := val.(int)
	return num, ok
}

// GetBool retrieves a bool value from the session
func GetBool(c *gin.Context, key string) (bool, bool) {
	val, exists := GetValue(c, key)
	if !exists {
		return false, false
	}
	
	b, ok := val.(bool)
	return b, ok
}

// Delete removes a value from the session
func Delete(c *gin.Context, key string) {
	session := Get(c)
	if session != nil {
		delete(session.Values, key)
		c.Set(ContextSessionKey, session)
	}
}

// Has checks if a key exists in the session
func Has(c *gin.Context, key string) bool {
	session := Get(c)
	if session != nil {
		_, exists := session.Values[key]
		return exists
	}
	return false
}

// Flash sets a one-time message in the session
// The message will be available for the current request and the next request
func Flash(c *gin.Context, key string, value interface{}) {
	Set(c, "_flash_"+key, value)
}

// GetFlash gets a flash message from the session and removes it
func GetFlash(c *gin.Context, key string) (interface{}, bool) {
	flashKey := "_flash_" + key
	val, exists := GetValue(c, flashKey)
	if exists {
		Delete(c, flashKey)
	}
	return val, exists
}

// FlashString sets a one-time string message in the session
func FlashString(c *gin.Context, key, value string) {
	Flash(c, key, value)
}

// GetFlashString gets a flash string message from the session and removes it
func GetFlashString(c *gin.Context, key string) (string, bool) {
	val, exists := GetFlash(c, key)
	if !exists {
		return "", false
	}
	
	str, ok := val.(string)
	return str, ok
}