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

	AuthFlag      = "Authenticated"
	AuthTimestamp = "AuthTimestamp"
	AuthNonce     = "AuthNonce"
)

type hmacAuthProvider struct {
	provider *hmacprovider.HMACProvider
}

func HMACAuth(provider *hmacprovider.HMACProvider) Provider {
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
			"client_ip": c.ClientIP(),
			"path":      c.Request.URL.Path,
			"method":    c.Request.Method,
		})

		return false
	}

	// Read request body for verification
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {

		logger.Error(err, "Failed to read request body", log.KV{
			"client_ip": c.ClientIP(),
		})

		return false
	}

	// Restore body for downstream handlers
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	// Verify HMAC signature
	bodyReader := bytes.NewReader(body)
	valid, err := h.provider.Verify256(bodyReader, hash, timestamp, nonce)

	if err != nil {

		logger.Warn("HMAC verification error", log.KV{
			"error":     err.Error(),
			"client_ip": c.ClientIP(),
			"path":      c.Request.URL.Path,
			"method":    c.Request.Method,
			"timestamp": timestamp,
		})

		return false
	}

	if !valid {

		logger.Info("HMAC verification failed - invalid signature", log.KV{
			"client_ip": c.ClientIP(),
			"path":      c.Request.URL.Path,
			"method":    c.Request.Method,
			"timestamp": timestamp,
		})

		return false
	}

	// Authentication successful
	logger.Debug("HMAC authentication successful", log.KV{
		"client_ip": c.ClientIP(),
		"path":      c.Request.URL.Path,
		"method":    c.Request.Method,
	})

	// Store authentication info in context for downstream handlers
	c.Set(AuthFlag, true)
	c.Set(AuthTimestamp, timestamp)
	c.Set(AuthNonce, nonce)

	return true
}
