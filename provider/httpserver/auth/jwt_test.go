package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGetJWTToken_CaseInsensitiveBearer(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantToken string
		wantValid bool
	}{
		{
			name:      "standard Bearer",
			header:    "Bearer mytoken123",
			wantToken: "mytoken123",
			wantValid: true,
		},
		{
			name:      "lowercase bearer",
			header:    "bearer mytoken123",
			wantToken: "mytoken123",
			wantValid: true,
		},
		{
			name:      "uppercase BEARER",
			header:    "BEARER mytoken123",
			wantToken: "mytoken123",
			wantValid: true,
		},
		{
			name:      "mixed case BeArEr",
			header:    "BeArEr mytoken123",
			wantToken: "mytoken123",
			wantValid: true,
		},
		{
			name:      "missing header",
			header:    "",
			wantToken: "",
			wantValid: false,
		},
		{
			name:      "wrong scheme",
			header:    "Basic dXNlcjpwYXNz",
			wantToken: "",
			wantValid: false,
		},
		{
			name:      "too short",
			header:    "Bear",
			wantToken: "",
			wantValid: false,
		},
		{
			name:      "no space after bearer",
			header:    "Bearer",
			wantToken: "",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req, _ := http.NewRequest("GET", "/test", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			c.Request = req

			token, valid := GetJWTToken(c)
			assert.Equal(t, tt.wantValid, valid)
			assert.Equal(t, tt.wantToken, token)
		})
	}
}
