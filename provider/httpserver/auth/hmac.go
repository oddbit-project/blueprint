package auth

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/hmacprovider"
	httplog "github.com/oddbit-project/blueprint/provider/httpserver/log"
	"io"
)

const (
	HeaderHMACHash      = "X-HMAC-Hash"
	HeaderHMACTimestamp = "X-HMAC-Timestamp"
	HeaderHMACNonce     = "X-HMAC-Nonce"

	HMACKeyId     = "HMACKeyId"
	HMACTimestamp = "HMACTimestamp"
	HMACNonce     = "HMACNonce"
	DefaultKeyId  = "authenticated"
)

type hmacAuthProvider struct {
	provider *hmacprovider.HMACProvider
}

func NewHMACAuthProvider(provider *hmacprovider.HMACProvider) Provider {
	return &hmacAuthProvider{
		provider: provider,
	}
}

func (h *hmacAuthProvider) CanAccess(c *gin.Context) bool {

	logger := httplog.GetRequestLogger(c)

	// Extract HMAC signature components from headers
	hash := c.GetHeader(HeaderHMACHash)
	timestamp := c.GetHeader(HeaderHMACTimestamp)
	nonce := c.GetHeader(HeaderHMACNonce)

	// Validate required headers
	if hash == "" || timestamp == "" || nonce == "" {
		logger.Warn("HMAC authentication failed: missing headers", log.KV{
			"clientIp": c.ClientIP(),
			"path":     c.Request.URL.Path,
			"method":   c.Request.Method,
		})

		return false
	}

	// Read request body for verification
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.Error(err, "Failed to read request body", log.KV{
			"clientIp": c.ClientIP(),
		})
		return false
	}

	// Restore body for downstream handlers
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	// Verify HMAC signature
	bodyReader := bytes.NewReader(body)
	keyId, valid, err := h.provider.Verify256(bodyReader, hash, timestamp, nonce)

	if err != nil {

		logger.Warn("HMAC verification error", log.KV{
			"error":     err.Error(),
			"clientIp":  c.ClientIP(),
			"path":      c.Request.URL.Path,
			"method":    c.Request.Method,
			"timestamp": timestamp,
		})

		return false
	}

	if !valid {

		logger.Info("HMAC verification failed - invalid signature", log.KV{
			"clientIp":  c.ClientIP(),
			"path":      c.Request.URL.Path,
			"method":    c.Request.Method,
			"timestamp": timestamp,
		})

		return false
	}

	// Authentication successful
	logger.Debug("HMAC authentication successful", log.KV{
		"keyId":    keyId,
		"clientIp": c.ClientIP(),
		"path":     c.Request.URL.Path,
		"method":   c.Request.Method,
	})

	// Store authentication info in context for downstream handlers
	if keyId == "" {
		keyId = DefaultKeyId
	}
	c.Set(HMACKeyId, keyId)
	c.Set(HMACTimestamp, timestamp)
	c.Set(HMACNonce, nonce)

	return true
}

// GetHMACIdentity fetch hmac keyId
func GetHMACIdentity(c *gin.Context) (string, bool) {
	keyId, exists := c.Get(HMACKeyId)
	if !exists {
		return "", false
	}
	return keyId.(string), true
}

// GetHMACDetails fetch hmac details
// returns keyId, timestamp, nonce, true if success
func GetHMACDetails(c *gin.Context) (string, string, string, bool) {
	keyId, exists := c.Get(HMACKeyId)
	if !exists {
		return "", "", "", false
	}
	ts, exists := c.Get(HMACTimestamp)
	if !exists {
		return "", "", "", false
	}
	nonce, exists := c.Get(HMACNonce)
	if !exists {
		return "", "", "", false
	}
	return keyId.(string), ts.(string), nonce.(string), true
}
