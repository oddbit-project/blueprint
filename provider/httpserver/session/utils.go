package session

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
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

// MarshallSessionData converts a session data object to JSON
func MarshallSessionData(session *SessionData) (string, error) {
	data, err := json.Marshal(session)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// UnmarshallSessionData converts JSON to a session data object
func UnmarshallSessionData(data string) (*SessionData, error) {
	var session SessionData
	err := json.Unmarshal([]byte(data), &session)
	if err != nil {
		return nil, err
	}
	return &session, nil
}
