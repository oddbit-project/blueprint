package main

import (
	"context"
	"crypto/x509"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver"
	"github.com/oddbit-project/blueprint/provider/httpserver/response"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
)

func main() {
	// Setup logger
	logger := log.New("mtls-server")
	logger.Info("Starting mTLS server demo...")

	// Configure mTLS server
	serverConfig := &httpserver.ServerConfig{
		Host: "localhost",
		Port: 8444,
		ServerConfig: tlsProvider.ServerConfig{
			TLSEnable: true,
			// Server certificate and key
			TLSCert: "../certs/server.crt",
			TLSKey:  "../certs/server.key",
			// CA certificates to validate client certificates
			TLSAllowedCACerts: []string{
				"../certs/ca.crt",
			},
			// Optional: Restrict allowed client DNS names (commented out for demo)
			// TLSAllowedDNSNames: []string{
			//	"demo-client.example.com",
			//	"client.blueprint.demo",
			// },
			// Security settings - use TLS 1.3 for maximum security
			TLSMinVersion: "TLS13",
			TLSMaxVersion: "TLS13",
			// Use strong cipher suites
			TLSCipherSuites: []string{
				"TLS_AES_256_GCM_SHA384",
				"TLS_CHACHA20_POLY1305_SHA256",
				"TLS_AES_128_GCM_SHA256",
			},
		},
	}

	// Create server with mTLS configuration
	server, err := serverConfig.NewServer(logger)
	if err != nil {
		logger.Fatal(err, "Failed to create server")
	}

	// Setup routes
	setupRoutes(server, logger)

	// Setup graceful shutdown
	setupGracefulShutdown(server, logger)

	// Start server
	logger.Info("mTLS server starting", log.KV{
		"host": serverConfig.Host,
		"port": serverConfig.Port,
		"tls":  serverConfig.TLSEnable,
	})

	if err := server.Start(); err != nil {
		logger.Fatal(err, "Server failed to start")
	}
}

func setupRoutes(server *httpserver.Server, logger *log.Logger) {
	// Add mTLS security logger middleware
	server.AddMiddleware(mTLSSecurityLogger(logger))

	// Public endpoint (no client certificate validation)
	server.Route().GET("/health", func(c *gin.Context) {
		response.Success(c, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
			"server":    "mTLS Demo Server",
		})
	})

	// Protected endpoint requiring client certificate
	server.Route().GET("/secure", mTLSAuthorizationMiddleware(logger), func(c *gin.Context) {
		// Get client certificate from context
		clientCert, exists := c.Get("client_cert")
		if !exists {
			c.JSON(500, gin.H{"error": "Internal error: client certificate not found in context"})
			return
		}

		cert := clientCert.(*x509.Certificate)
		clientInfo := extractClientInfo(cert)

		response.Success(c, gin.H{
			"message":     "Access granted to secure endpoint",
			"client_info": clientInfo,
			"timestamp":   time.Now().Format(time.RFC3339),
		})
	})

	// API endpoints with different authorization levels
	api := server.Group("/api/v1")
	api.Use(mTLSAuthorizationMiddleware(logger))
	{
		api.GET("/user/profile", func(c *gin.Context) {
			clientDN, _ := c.Get("client_dn")
			response.Success(c, gin.H{
				"user_id":    "demo_user_123",
				"username":   "demo_user",
				"email":      "demo@example.com",
				"client_dn":  clientDN,
				"privileges": []string{"read", "write"},
			})
		})

		api.POST("/data", func(c *gin.Context) {
			var requestData map[string]interface{}
			if err := c.ShouldBindJSON(&requestData); err != nil {
				c.JSON(400, gin.H{"error": "Invalid JSON payload"})
				return
			}

			clientDN, _ := c.Get("client_dn")
			response.Success(c, gin.H{
				"message":   "Data processed successfully",
				"data_id":   fmt.Sprintf("data_%d", time.Now().Unix()),
				"client_dn": clientDN,
				"received":  requestData,
			})
		})

		api.GET("/admin/stats", func(c *gin.Context) {
			// Only allow specific client organizations for admin endpoints
			clientCert, _ := c.Get("client_cert")
			cert := clientCert.(*x509.Certificate)

			if !isAdminClient(cert) {
				c.JSON(403, gin.H{"error": "Admin access required"})
				return
			}

			response.Success(c, gin.H{
				"active_connections": 42,
				"uptime_seconds":     3600,
				"memory_usage_mb":    128,
				"admin_client":       cert.Subject.String(),
			})
		})
	}
}

func mTLSSecurityLogger(logger *log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)

		if c.Request.TLS != nil && len(c.Request.TLS.PeerCertificates) > 0 {
			clientCert := c.Request.TLS.PeerCertificates[0]

			logger.Info("mTLS request", log.KV{
				"client_dn":      clientCert.Subject.String(),
				"client_serial":  clientCert.SerialNumber.String(),
				"path":           c.Request.URL.Path,
				"method":         c.Request.Method,
				"status":         c.Writer.Status(),
				"client_ip":      c.ClientIP(),
				"duration_ms":    duration.Milliseconds(),
				"tls_version":    getTLSVersion(c.Request.TLS.Version),
				"cipher_suite":   getCipherSuite(c.Request.TLS.CipherSuite),
				"user_agent":     c.Request.UserAgent(),
			})
		} else {
			logger.Info("Non-mTLS request", log.KV{
				"path":        c.Request.URL.Path,
				"method":      c.Request.Method,
				"status":      c.Writer.Status(),
				"client_ip":   c.ClientIP(),
				"duration_ms": duration.Milliseconds(),
				"user_agent":  c.Request.UserAgent(),
			})
		}
	}
}

func mTLSAuthorizationMiddleware(logger *log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.TLS == nil || len(c.Request.TLS.PeerCertificates) == 0 {
			logger.Warn("Client certificate required but not provided", log.KV{
				"path":      c.Request.URL.Path,
				"client_ip": c.ClientIP(),
			})
			c.AbortWithStatusJSON(401, gin.H{"error": "Client certificate required for this endpoint"})
			return
		}

		clientCert := c.Request.TLS.PeerCertificates[0]

		// Validate certificate is still valid
		now := time.Now()
		if now.Before(clientCert.NotBefore) || now.After(clientCert.NotAfter) {
			logger.Warn("Client certificate expired or not yet valid", log.KV{
				"client_dn":  clientCert.Subject.String(),
				"not_before": clientCert.NotBefore,
				"not_after":  clientCert.NotAfter,
				"now":        now,
			})
			c.AbortWithStatusJSON(401, gin.H{"error": "Client certificate expired or not yet valid"})
			return
		}

		// Custom authorization logic based on certificate attributes
		if !isAuthorizedClient(clientCert) {
			logger.Warn("Client certificate not authorized", log.KV{
				"client_dn":     clientCert.Subject.String(),
				"client_serial": clientCert.SerialNumber.String(),
				"organizations": clientCert.Subject.Organization,
			})
			c.AbortWithStatusJSON(403, gin.H{"error": "Client certificate not authorized"})
			return
		}

		// Store client identity in context for downstream handlers
		c.Set("client_cert", clientCert)
		c.Set("client_dn", clientCert.Subject.String())
		c.Set("client_serial", clientCert.SerialNumber.String())

		logger.Debug("mTLS client authorized", log.KV{
			"client_dn":     clientCert.Subject.String(),
			"client_serial": clientCert.SerialNumber.String(),
			"path":          c.Request.URL.Path,
		})

		c.Next()
	}
}

func isAuthorizedClient(cert *x509.Certificate) bool {
	// Allow clients from specific organizations
	authorizedOrgs := []string{"Blueprint Demo"}

	for _, org := range cert.Subject.Organization {
		for _, authorizedOrg := range authorizedOrgs {
			if org == authorizedOrg {
				return true
			}
		}
	}
	return false
}

func isAdminClient(cert *x509.Certificate) bool {
	// Admin access requires specific OU
	for _, ou := range cert.Subject.OrganizationalUnit {
		if ou == "Admin" || ou == "Client" { // For demo purposes, allow Client OU as admin
			return true
		}
	}
	return false
}

func extractClientInfo(cert *x509.Certificate) map[string]interface{} {
	return map[string]interface{}{
		"subject":      cert.Subject.String(),
		"issuer":       cert.Issuer.String(),
		"serial":       cert.SerialNumber.String(),
		"not_before":   cert.NotBefore.Format(time.RFC3339),
		"not_after":    cert.NotAfter.Format(time.RFC3339),
		"dns_names":    cert.DNSNames,
		"ip_addresses": cert.IPAddresses,
		"organizations": cert.Subject.Organization,
		"organizational_units": cert.Subject.OrganizationalUnit,
		"common_name":  cert.Subject.CommonName,
	}
}

func getTLSVersion(version uint16) string {
	switch version {
	case 0x0303:
		return "TLS 1.2"
	case 0x0304:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("Unknown (0x%04x)", version)
	}
}

func getCipherSuite(suite uint16) string {
	switch suite {
	case 0x1301:
		return "TLS_AES_128_GCM_SHA256"
	case 0x1302:
		return "TLS_AES_256_GCM_SHA384"
	case 0x1303:
		return "TLS_CHACHA20_POLY1305_SHA256"
	default:
		return fmt.Sprintf("Unknown (0x%04x)", suite)
	}
}

func setupGracefulShutdown(server *httpserver.Server, logger *log.Logger) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		logger.Info("Shutting down mTLS server...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			logger.Error(err, "Error during server shutdown")
		} else {
			logger.Info("mTLS server shutdown complete")
		}
		os.Exit(0)
	}()
}