package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGetHMACIdentity_TypeSafety(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name      string
		setup     func(c *gin.Context)
		wantID    string
		wantFound bool
	}{
		{
			name:      "key not set",
			setup:     func(c *gin.Context) {},
			wantID:    "",
			wantFound: false,
		},
		{
			name: "valid string key",
			setup: func(c *gin.Context) {
				c.Set(HMACKeyId, "test-key")
			},
			wantID:    "test-key",
			wantFound: true,
		},
		{
			name: "non-string value does not panic",
			setup: func(c *gin.Context) {
				c.Set(HMACKeyId, 12345)
			},
			wantID:    "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest("GET", "/", nil)
			tt.setup(c)

			id, found := GetHMACIdentity(c)
			assert.Equal(t, tt.wantID, id)
			assert.Equal(t, tt.wantFound, found)
		})
	}
}

func TestGetHMACDetails_TypeSafety(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name      string
		setup     func(c *gin.Context)
		wantKey   string
		wantTS    string
		wantNonce string
		wantFound bool
	}{
		{
			name:      "nothing set",
			setup:     func(c *gin.Context) {},
			wantFound: false,
		},
		{
			name: "all valid strings",
			setup: func(c *gin.Context) {
				c.Set(HMACKeyId, "key1")
				c.Set(HMACTimestamp, "12345")
				c.Set(HMACNonce, "nonce1")
			},
			wantKey:   "key1",
			wantTS:    "12345",
			wantNonce: "nonce1",
			wantFound: true,
		},
		{
			name: "non-string keyId does not panic",
			setup: func(c *gin.Context) {
				c.Set(HMACKeyId, 999)
				c.Set(HMACTimestamp, "12345")
				c.Set(HMACNonce, "nonce1")
			},
			wantFound: false,
		},
		{
			name: "non-string timestamp does not panic",
			setup: func(c *gin.Context) {
				c.Set(HMACKeyId, "key1")
				c.Set(HMACTimestamp, 12345)
				c.Set(HMACNonce, "nonce1")
			},
			wantFound: false,
		},
		{
			name: "non-string nonce does not panic",
			setup: func(c *gin.Context) {
				c.Set(HMACKeyId, "key1")
				c.Set(HMACTimestamp, "12345")
				c.Set(HMACNonce, 999)
			},
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest("GET", "/", nil)
			tt.setup(c)

			key, ts, nonce, found := GetHMACDetails(c)
			assert.Equal(t, tt.wantFound, found)
			if tt.wantFound {
				assert.Equal(t, tt.wantKey, key)
				assert.Equal(t, tt.wantTS, ts)
				assert.Equal(t, tt.wantNonce, nonce)
			}
		})
	}
}
