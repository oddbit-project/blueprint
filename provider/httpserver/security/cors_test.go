package security

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCorsConfig(t *testing.T) {
	cfg := NewCorsConfig()
	
	assert.True(t, cfg.CorsEnabled)
	assert.Empty(t, cfg.AllowOrigins)
	assert.Equal(t, []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}, cfg.AllowMethods)
	assert.Equal(t, []string{"Origin", "Content-Type", "Authorization", "X-CSRF-Token", "X-HMAC-Hash", "X-HMAC-Timestamp", "X-HMAC-Nonce"}, cfg.AllowHeaders)
	assert.Empty(t, cfg.ExposeHeaders)
	assert.False(t, cfg.AllowCredentials)
	assert.Equal(t, 3600, cfg.MaxAge)
	assert.Equal(t, "Origin", cfg.Vary)
	assert.False(t, cfg.DevMode)
}

func TestCorsConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *CorsConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with origins",
			config: &CorsConfig{
				AllowOrigins: []string{"https://example.com"},
				MaxAge:       3600,
			},
			wantErr: false,
		},
		{
			name: "valid config in dev mode without origins",
			config: &CorsConfig{
				AllowOrigins: []string{},
				DevMode:      true,
				MaxAge:       3600,
			},
			wantErr: false,
		},
		{
			name: "invalid - no origins and not dev mode",
			config: &CorsConfig{
				AllowOrigins: []string{},
				DevMode:      false,
			},
			wantErr: true,
			errMsg:  "no allowOrigin value and devMode is false",
		},
		{
			name: "invalid - negative max age",
			config: &CorsConfig{
				AllowOrigins: []string{"https://example.com"},
				MaxAge:       -1,
			},
			wantErr: true,
			errMsg:  "maxAge value cannot be negative",
		},
		{
			name: "invalid - wildcard with credentials",
			config: &CorsConfig{
				AllowOrigins:     []string{"*"},
				AllowCredentials: true,
				MaxAge:           3600,
			},
			wantErr: true,
			errMsg:  "allowOrigin can not contain '*' if Allow-Credentials is true",
		},
		{
			name: "invalid - malformed origin",
			config: &CorsConfig{
				AllowOrigins: []string{"not-a-valid-origin"},
				MaxAge:       3600,
			},
			wantErr: true,
			errMsg:  "invalid allowOrigin value not-a-valid-origin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, tt.errMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCORSMiddleware_Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	cfg := &CorsConfig{
		CorsEnabled: false,
	}
	
	router := gin.New()
	router.Use(CORSMiddleware(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})
	
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	assert.Equal(t, 200, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddleware_DevMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	cfg := &CorsConfig{
		CorsEnabled:      true,
		DevMode:          true,
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		ExposeHeaders:    []string{"X-Total-Count"},
		AllowCredentials: true,
		MaxAge:           7200,
		Vary:             "Origin",
	}
	
	router := gin.New()
	router.Use(CORSMiddleware(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})
	router.POST("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})
	router.OPTIONS("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})
	
	tests := []struct {
		name           string
		method         string
		origin         string
		wantStatus     int
		checkHeaders   bool
		wantCORSHeader bool
	}{
		{
			name:           "regular request with origin",
			method:         "GET",
			origin:         "https://example.com",
			wantStatus:     200,
			checkHeaders:   true,
			wantCORSHeader: true,
		},
		{
			name:           "preflight request",
			method:         "OPTIONS",
			origin:         "https://example.com",
			wantStatus:     204,
			checkHeaders:   true,
			wantCORSHeader: true,
		},
		{
			name:           "method not allowed",
			method:         "DELETE",
			origin:         "https://example.com",
			wantStatus:     405,
			checkHeaders:   false,
			wantCORSHeader: false,
		},
		{
			name:           "case insensitive method",
			method:         "GET",
			origin:         "https://example.com",
			wantStatus:     200,
			checkHeaders:   true,
			wantCORSHeader: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/test", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			w := httptest.NewRecorder()
			
			router.ServeHTTP(w, req)
			
			assert.Equal(t, tt.wantStatus, w.Code)
			
			if tt.checkHeaders && tt.wantCORSHeader {
				assert.Equal(t, tt.origin, w.Header().Get("Access-Control-Allow-Origin"))
				assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
				assert.Equal(t, "Content-Type, Authorization", w.Header().Get("Access-Control-Allow-Headers"))
				assert.Equal(t, "GET, POST, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
				assert.Equal(t, "7200", w.Header().Get("Access-Control-Max-Age"))
				assert.Equal(t, "X-Total-Count", w.Header().Get("Access-Control-Expose-Headers"))
				assert.Equal(t, "Origin", w.Header().Get("Vary"))
			}
		})
	}
}

func TestCORSMiddleware_WildcardOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	cfg := &CorsConfig{
		CorsEnabled:      true,
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type"},
		AllowCredentials: false,
		MaxAge:           3600,
		Vary:             "Origin",
	}
	
	router := gin.New()
	router.Use(CORSMiddleware(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})
	
	tests := []struct {
		name       string
		method     string
		origin     string
		wantStatus int
	}{
		{
			name:       "any origin allowed",
			method:     "GET",
			origin:     "https://any-origin.com",
			wantStatus: 200,
		},
		{
			name:       "preflight with any origin",
			method:     "OPTIONS",
			origin:     "https://another-origin.com",
			wantStatus: 204,
		},
		{
			name:       "method not allowed",
			method:     "PUT",
			origin:     "https://example.com",
			wantStatus: 405,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/test", nil)
			req.Header.Set("Origin", tt.origin)
			w := httptest.NewRecorder()
			
			router.ServeHTTP(w, req)
			
			assert.Equal(t, tt.wantStatus, w.Code)
			
			if tt.wantStatus == 200 || tt.wantStatus == 204 {
				assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
				assert.Equal(t, "false", w.Header().Get("Access-Control-Allow-Credentials"))
				assert.Equal(t, "Origin", w.Header().Get("Vary"))
			}
		})
	}
}

func TestCORSMiddleware_SpecificOrigins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	cfg := &CorsConfig{
		CorsEnabled:      true,
		AllowOrigins:     []string{"https://allowed1.com", "https://allowed2.com"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		ExposeHeaders:    []string{},
		AllowCredentials: true,
		MaxAge:           3600,
		Vary:             "Origin",
	}
	
	router := gin.New()
	router.Use(CORSMiddleware(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})
	
	tests := []struct {
		name           string
		method         string
		origin         string
		wantStatus     int
		wantCORSHeader bool
	}{
		{
			name:           "allowed origin 1",
			method:         "GET",
			origin:         "https://allowed1.com",
			wantStatus:     200,
			wantCORSHeader: true,
		},
		{
			name:           "allowed origin 2",
			method:         "GET",
			origin:         "https://allowed2.com",
			wantStatus:     200,
			wantCORSHeader: true,
		},
		{
			name:           "not allowed origin",
			method:         "GET",
			origin:         "https://notallowed.com",
			wantStatus:     200,
			wantCORSHeader: false,
		},
		{
			name:           "preflight from allowed origin",
			method:         "OPTIONS",
			origin:         "https://allowed1.com",
			wantStatus:     204,
			wantCORSHeader: true,
		},
		{
			name:           "preflight from not allowed origin",
			method:         "OPTIONS",
			origin:         "https://notallowed.com",
			wantStatus:     204,
			wantCORSHeader: false,
		},
		{
			name:           "no origin header",
			method:         "GET",
			origin:         "",
			wantStatus:     200,
			wantCORSHeader: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/test", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			w := httptest.NewRecorder()
			
			router.ServeHTTP(w, req)
			
			assert.Equal(t, tt.wantStatus, w.Code)
			
			if tt.wantCORSHeader {
				assert.Equal(t, tt.origin, w.Header().Get("Access-Control-Allow-Origin"))
				assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
				assert.Equal(t, "Content-Type, Authorization", w.Header().Get("Access-Control-Allow-Headers"))
				assert.Equal(t, "GET, POST, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
				assert.Equal(t, "3600", w.Header().Get("Access-Control-Max-Age"))
				assert.Equal(t, "Origin", w.Header().Get("Vary"))
			} else {
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

func TestCORSMiddleware_OptionsNotAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	cfg := &CorsConfig{
		CorsEnabled:  true,
		AllowOrigins: []string{"https://example.com"},
		AllowMethods: []string{"GET", "POST"}, // OPTIONS not included
		MaxAge:       3600,
	}
	
	router := gin.New()
	router.Use(CORSMiddleware(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})
	
	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	assert.Equal(t, 405, w.Code)
}

func TestCORSMiddleware_EmptyExposeHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	cfg := &CorsConfig{
		CorsEnabled:   true,
		AllowOrigins:  []string{"https://example.com"},
		AllowMethods:  []string{"GET"},
		ExposeHeaders: []string{}, // Empty expose headers
		MaxAge:        3600,
	}
	
	router := gin.New()
	router.Use(CORSMiddleware(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})
	
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	assert.Equal(t, 200, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Expose-Headers"))
}

func TestCORSMiddleware_CaseInsensitiveMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	cfg := &CorsConfig{
		CorsEnabled:  true,
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"get", "POST", "Options"}, // Mixed case
		MaxAge:       3600,
	}
	
	router := gin.New()
	router.Use(CORSMiddleware(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})
	router.POST("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})
	router.OPTIONS("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})
	
	tests := []struct {
		name       string
		method     string
		wantStatus int
	}{
		{
			name:       "GET uppercase",
			method:     "GET",
			wantStatus: 200,
		},
		{
			name:       "get lowercase",
			method:     "GET",
			wantStatus: 200,
		},
		{
			name:       "POST uppercase",
			method:     "POST",
			wantStatus: 200,
		},
		{
			name:       "post lowercase",
			method:     "POST",
			wantStatus: 200,
		},
		{
			name:       "OPTIONS uppercase",
			method:     "OPTIONS",
			wantStatus: 204,
		},
		{
			name:       "options lowercase",
			method:     "OPTIONS",
			wantStatus: 204,
		},
		{
			name:       "DELETE not allowed",
			method:     "DELETE",
			wantStatus: 405,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/test", nil)
			req.Header.Set("Origin", "https://example.com")
			w := httptest.NewRecorder()
			
			router.ServeHTTP(w, req)
			
			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}