package fingerprint

import (
	"encoding/gob"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/provider/httpserver/response"
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
)

var fingerprintGobOnce sync.Once

// RegisterGobTypes registers fingerprint types with the gob encoder.
// Safe to call multiple times via sync.Once.
func RegisterGobTypes() {
	fingerprintGobOnce.Do(func() {
		gob.Register(&DeviceFingerprint{})
	})
}

const (
	FingerprintKey = "_fingerprint_"
)

// FingerprintStore abstracts fingerprint storage.
// Implement this interface to provide custom storage (e.g. database, Redis).
type FingerprintStore interface {
	// Load retrieves a stored fingerprint for the current request, or nil if none exists.
	Load(c *gin.Context) *DeviceFingerprint
	// Save stores a fingerprint for the current request.
	Save(c *gin.Context, fp *DeviceFingerprint)
}

// SessionFingerprintStore implements FingerprintStore using session storage.
type SessionFingerprintStore struct{}

func (s *SessionFingerprintStore) Load(c *gin.Context) *DeviceFingerprint {
	sess := session.Get(c)
	if sess == nil {
		return nil
	}
	result, exists := sess.Get(FingerprintKey)
	if !exists {
		return nil
	}
	fp, ok := result.(*DeviceFingerprint)
	if !ok {
		return nil
	}
	return fp
}

func (s *SessionFingerprintStore) Save(c *gin.Context, fp *DeviceFingerprint) {
	sess := session.Get(c)
	if sess != nil {
		sess.Set(FingerprintKey, fp)
	}
}

// FingerprintMiddleware creates a middleware that validates device fingerprints
// using the provided store for loading existing fingerprints.
func FingerprintMiddleware(generator *Generator, store FingerprintStore, strict bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		existing := store.Load(c)
		if existing != nil {
			current := generator.Generate(c)
			if !generator.Compare(existing, current, strict) {
				response.Http401(c)
				return
			}
		}

		c.Next()
	}
}

// SessionFingerprintMiddleware creates a middleware that validates device fingerprints
// stored in the session. This is a convenience wrapper around FingerprintMiddleware
// with SessionFingerprintStore.
func SessionFingerprintMiddleware(generator *Generator, strict bool) gin.HandlerFunc {
	RegisterGobTypes()
	return FingerprintMiddleware(generator, &SessionFingerprintStore{}, strict)
}

// GetFingerprint fetch fingerprint from session if exists
func GetFingerprint(c *gin.Context) *DeviceFingerprint {
	store := &SessionFingerprintStore{}
	return store.Load(c)
}

// UpdateFingerprint stores or updates fingerprint in the session.
// This should be called during the login process.
func UpdateFingerprint(s *session.SessionData, fp *DeviceFingerprint) {
	s.Set(FingerprintKey, fp)
}
