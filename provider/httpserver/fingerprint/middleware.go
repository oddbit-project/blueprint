package fingerprint

import (
	"encoding/gob"
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/provider/httpserver/response"
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
)

const (
	FingerprintKey = "_fingerprint_"
)

func SessionFingerprintMiddleware(generator *Generator, strict bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// fingerprint match is triggered only if session exists
		sess := session.Get(c)
		if sess != nil {
			// and if fingerprint already exists
			existing := GetFingerprint(c)
			if existing != nil {
				// fingerprint exists, lets match
				current := generator.Generate(c)
				if !generator.Compare(existing, current, strict) {
					response.Http401(c)
					return
				}
			}
		}

		c.Next()
	}
}

// GetFingerprint fetch fingerprint from session if exists
func GetFingerprint(c *gin.Context) *DeviceFingerprint {
	result, exists := c.Get(FingerprintKey)
	if !exists {
		return nil
	}
	return result.(*DeviceFingerprint)
}

// UpdateFingerprint stores or updates fingerprint in the session
// this should be called during the login process
func UpdateFingerprint(s *session.SessionData, fp *DeviceFingerprint) {
	s.Set(FingerprintKey, fp)
}

func init() {
	gob.Register(&DeviceFingerprint{})
}
