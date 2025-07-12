package session

import (
	"time"
)

// SessionData represents the session data stored in a backend
type SessionData struct {
	Values       map[string]any
	LastAccessed time.Time
	Created      time.Time
	ID           string
}

// Set stores a value in the session
func (s *SessionData) Set(key string, value any) {
	s.Values[key] = value
}

// Get retrieves a value from the session
func (s *SessionData) Get(key string) (any, bool) {
	v, ok := s.Values[key]
	return v, ok
}

// GetString retrieves a string value from the session
func (s *SessionData) GetString(key string) (string, bool) {
	v, ok := s.Values[key]
	if ok {
		cast, ok := v.(string)
		return cast, ok
	}
	return "", false
}

// GetInt retrieves an int value from the session
func (s *SessionData) GetInt(key string) (int, bool) {
	v, ok := s.Values[key]
	if ok {
		cast, ok := v.(int)
		return cast, ok
	}
	return 0, false
}

// GetBool retrieves a bool value from the session
func (s *SessionData) GetBool(key string) (bool, bool) {
	v, ok := s.Values[key]
	if ok {
		cast, ok := v.(bool)
		return cast, ok
	}
	return false, false
}

// Delete removes a value from the session
func (s *SessionData) Delete(key string) {
	delete(s.Values, key)
}

// Has checks if a key exists in the session
func (s *SessionData) Has(key string) bool {
	_, ok := s.Values[key]
	return ok
}

// Flash sets a one-time message in the session
// The message will be available for the current request and the next request
func (s *SessionData) Flash(value any) {
	s.Values["_flash_"] = value
}

// GetFlash gets a flash message from the session and removes it
func (s *SessionData) GetFlash() (interface{}, bool) {
	v, ok := s.Values["_flash_"]
	if ok {
		delete(s.Values, "_flash_")
	}
	return v, ok
}

// FlashString sets a one-time string message in the session
func (s *SessionData) FlashString(value string) {
	s.Flash(value)
}

// GetFlashString gets a flash string message from the session and removes it
func (s *SessionData) GetFlashString() (string, bool) {
	val, exists := s.GetFlash()
	if !exists {
		return "", false
	}

	str, ok := val.(string)
	return str, ok
}
