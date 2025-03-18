package session

import (
	"encoding/base64"
	"encoding/json"
	"github.com/google/uuid"
)

// generateSessionID creates a random session ID
func generateSessionID() string {
	return base64.URLEncoding.EncodeToString([]byte(uuid.New().String()))
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
