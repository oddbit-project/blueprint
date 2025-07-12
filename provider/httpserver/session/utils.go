package session

import (
	"crypto/rand"
	"encoding/base64"
)

// generateSessionID creates a random session ID
func generateSessionID() string {
	buf := make([]byte, 128)
	_, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(buf)
}

// GenerateSessionID creates a random session ID (public version for external use)
func GenerateSessionID() string {
	return generateSessionID()
}
