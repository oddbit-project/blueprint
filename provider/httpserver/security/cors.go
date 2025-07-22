package security

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver/request"
	"slices"
	"strconv"
	"strings"
)

type CorsConfig struct {
	CorsEnabled      bool     `json:"corsEnabled""`
	AllowOrigins     []string `json:"allowOrigins"`
	AllowMethods     []string `json:"allowMethods"`
	AllowHeaders     []string `json:"allowHeaders"`
	ExposeHeaders    []string `json:"exposeHeaders"`
	AllowCredentials bool     `json:"allowCredentials"`
	MaxAge           int      `json:"maxAgeSeconds"`
	Vary             string   `json:"vary"`
	DevMode          bool     `json:"devMode"`
}

func NewCorsConfig() *CorsConfig {
	return &CorsConfig{
		CorsEnabled:      true,
		AllowOrigins:     []string{},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-CSRF-Token", "X-HMAC-Hash", "X-HMAC-Timestamp", "X-HMAC-Nonce"},
		ExposeHeaders:    []string{},
		AllowCredentials: false,
		MaxAge:           3600,
		Vary:             "Origin",
		DevMode:          false,
	}
}

func (c *CorsConfig) Validate() error {
	if len(c.AllowOrigins) == 0 && !c.DevMode {
		return errors.New("no allowOrigin value and devMode is false")
	}
	if c.MaxAge < 0 {
		return errors.New("maxAge value cannot be negative")
	}
	if c.AllowCredentials {
		if slices.Contains(c.AllowOrigins, "*") {
			return errors.New("allowOrigin can not contain '*' if Allow-Credentials is true")
		}
	}

	// validate origins
	for _, origin := range c.AllowOrigins {
		if !request.ValidOrigin(origin, []string{"http", "https"}) {
			return errors.New(fmt.Sprintf("invalid allowOrigin value %s", origin))
		}
	}

	return nil
}

func CORSMiddleware(cfg *CorsConfig) gin.HandlerFunc {
	if !cfg.CorsEnabled {
		// CORS disabled, dummy middleware
		return func(c *gin.Context) {
			c.Next()
		}
	}

	// Normalize allowed methods to uppercase for case-insensitive comparison
	normalizedMethods := make([]string, len(cfg.AllowMethods))
	for i, method := range cfg.AllowMethods {
		normalizedMethods[i] = strings.ToUpper(method)
	}

	methods := strings.Join(cfg.AllowMethods, ", ")
	headers := strings.Join(cfg.AllowHeaders, ", ")
	exposeHeaders := strings.Join(cfg.ExposeHeaders, ", ")
	allowCredentials := strconv.FormatBool(cfg.AllowCredentials)
	maxAge := strconv.Itoa(cfg.MaxAge)

	if cfg.DevMode {
		// Development middleware - use dynamic allow-origin
		return func(c *gin.Context) {
			log.FromContext(c).Warn("CORS DevMode: dynamic Access-Control-Allow-Origin enabled! >>>>>>>>>>   DO NOT USE IN PRODUCTION   <<<<<<<<<")
			origin := c.Request.Header.Get("Origin")
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", allowCredentials)
			c.Writer.Header().Set("Access-Control-Allow-Headers", headers)
			c.Writer.Header().Set("Access-Control-Allow-Methods", methods)
			c.Writer.Header().Set("Access-Control-Max-Age", maxAge)
			if len(exposeHeaders) > 0 {
				c.Writer.Header().Set("Access-Control-Expose-Headers", exposeHeaders)
			}
			c.Writer.Header().Set("Vary", cfg.Vary)

			if c.Request.Method == "OPTIONS" {
				// Only handle OPTIONS if it's in allowed methods
				if slices.Contains(normalizedMethods, "OPTIONS") {
					c.AbortWithStatus(204)
				} else {
					c.AbortWithStatus(405)
				}
				return
			}

			// Validate request method is allowed
			if !slices.Contains(normalizedMethods, strings.ToUpper(c.Request.Method)) {
				c.AbortWithStatus(405)
				return
			}

			c.Next()
		}
	}
	if slices.Contains(cfg.AllowOrigins, "*") {
		// Production middleware
		return func(c *gin.Context) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "false")
			c.Writer.Header().Set("Access-Control-Allow-Headers", headers)
			c.Writer.Header().Set("Access-Control-Allow-Methods", methods)
			c.Writer.Header().Set("Access-Control-Max-Age", maxAge)
			if len(exposeHeaders) > 0 {
				c.Writer.Header().Set("Access-Control-Expose-Headers", exposeHeaders)
			}
			c.Writer.Header().Set("Vary", cfg.Vary)

			if c.Request.Method == "OPTIONS" {
				// Only handle OPTIONS if it's in allowed methods
				if slices.Contains(normalizedMethods, "OPTIONS") {
					c.AbortWithStatus(204)
				} else {
					c.AbortWithStatus(405)
				}
				return
			}

			// Validate request method is allowed
			if !slices.Contains(normalizedMethods, strings.ToUpper(c.Request.Method)) {
				c.AbortWithStatus(405)
				return
			}

			c.Next()
		}
	}

	// Production middleware
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if slices.Contains(cfg.AllowOrigins, origin) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", allowCredentials)
			c.Writer.Header().Set("Access-Control-Allow-Headers", headers)
			c.Writer.Header().Set("Access-Control-Allow-Methods", methods)
			c.Writer.Header().Set("Access-Control-Max-Age", maxAge)
			if len(exposeHeaders) > 0 {
				c.Writer.Header().Set("Access-Control-Expose-Headers", exposeHeaders)
			}
			c.Writer.Header().Set("Vary", cfg.Vary)
		}

		if c.Request.Method == "OPTIONS" {
			// Only handle OPTIONS if it's in allowed methods
			if slices.Contains(normalizedMethods, "OPTIONS") {
				c.AbortWithStatus(204)
			} else {
				c.AbortWithStatus(405)
			}
			return
		}

		// Validate request method is allowed
		if !slices.Contains(normalizedMethods, strings.ToUpper(c.Request.Method)) {
			c.AbortWithStatus(405)
			return
		}

		c.Next()
	}
}
