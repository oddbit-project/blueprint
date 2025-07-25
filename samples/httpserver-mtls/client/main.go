package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/oddbit-project/blueprint/log"
)

// ClientConfig holds the mTLS client configuration
type ClientConfig struct {
	ServerURL  string
	CACert     string
	ClientCert string
	ClientKey  string
	Timeout    time.Duration
}

// MTLSClient represents an mTLS-enabled HTTP client following Blueprint patterns
type MTLSClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *log.Logger
	config     *ClientConfig
}

// NewMTLSClient creates a new mTLS client with proper configuration
func NewMTLSClient(config *ClientConfig, logger *log.Logger) (*MTLSClient, error) {
	if config == nil {
		return nil, fmt.Errorf("client config is required")
	}
	if logger == nil {
		logger = log.New("mtls-client")
	}

	// Set default timeout if not specified
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	// Load client certificate
	clientCert, err := tls.LoadX509KeyPair(config.ClientCert, config.ClientKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate: %w", err)
	}

	// Load CA certificate
	caCertPEM, err := os.ReadFile(config.CACert)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCertPEM) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	// Configure TLS with mTLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caCertPool,
		MinVersion:   tls.VersionTLS13, // Use TLS 1.3
		CipherSuites: []uint16{
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_AES_128_GCM_SHA256,
		},
	}

	// Create HTTP client with mTLS configuration
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: config.Timeout,
	}

	return &MTLSClient{
		baseURL:    strings.TrimSuffix(config.ServerURL, "/"),
		httpClient: httpClient,
		logger:     logger,
		config:     config,
	}, nil
}

// makeRequest creates and sends an mTLS HTTP request with proper logging
func (c *MTLSClient) makeRequest(method, path string, body interface{}) (*http.Response, error) {
	// Prepare request body
	var requestBody []byte
	var err error

	if body != nil {
		requestBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	// Create HTTP request
	url := c.baseURL + path
	req, err := http.NewRequest(method, url, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set standard headers
	req.Header.Set("User-Agent", "Blueprint-mTLS-Client/1.0")
	req.Header.Set("Accept", "application/json")
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Log request details
	c.logger.Debug("Making mTLS request", log.KV{
		"method": method,
		"url":    url,
		"path":   path,
	})

	start := time.Now()

	// Make the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error(err, "mTLS request failed", log.KV{
			"method":      method,
			"url":         url,
			"duration_ms": time.Since(start).Milliseconds(),
		})
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Log response details
	c.logger.Info("mTLS request completed", log.KV{
		"method":      method,
		"url":         url,
		"status":      resp.StatusCode,
		"duration_ms": time.Since(start).Milliseconds(),
	})

	return resp, nil
}

// Get performs an HTTP GET request
func (c *MTLSClient) Get(path string) (*http.Response, error) {
	return c.makeRequest("GET", path, nil)
}

// Post performs an HTTP POST request with JSON body
func (c *MTLSClient) Post(path string, body interface{}) (*http.Response, error) {
	return c.makeRequest("POST", path, body)
}

// Put performs an HTTP PUT request with JSON body
func (c *MTLSClient) Put(path string, body interface{}) (*http.Response, error) {
	return c.makeRequest("PUT", path, body)
}

// Delete performs an HTTP DELETE request
func (c *MTLSClient) Delete(path string) (*http.Response, error) {
	return c.makeRequest("DELETE", path, nil)
}

// GetJSON performs a GET request and unmarshals the JSON response
func (c *MTLSClient) GetJSON(path string, result interface{}) error {
	resp, err := c.Get(path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(result)
}

// PostJSON performs a POST request and unmarshals the JSON response
func (c *MTLSClient) PostJSON(path string, requestBody interface{}, result interface{}) error {
	resp, err := c.Post(path, requestBody)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(result)
}

// Close cleans up the client (implements io.Closer for resource management)
func (c *MTLSClient) Close() error {
	c.logger.Debug("Closing mTLS client")
	// HTTP client doesn't need explicit cleanup, but this allows for future extensions
	return nil
}

// Demo response structures
type HealthResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Status    string `json:"status"`
		Timestamp string `json:"timestamp"`
		Server    string `json:"server"`
	} `json:"data"`
}

type SecureResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Message     string                 `json:"message"`
		ClientInfo  map[string]interface{} `json:"client_info"`
		Timestamp   string                 `json:"timestamp"`
	} `json:"data"`
}

type UserProfileResponse struct {
	Success bool `json:"success"`
	Data    struct {
		UserID     string   `json:"user_id"`
		Username   string   `json:"username"`
		Email      string   `json:"email"`
		ClientDN   string   `json:"client_dn"`
		Privileges []string `json:"privileges"`
	} `json:"data"`
}

type DataResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Message  string                 `json:"message"`
		DataID   string                 `json:"data_id"`
		ClientDN string                 `json:"client_dn"`
		Received map[string]interface{} `json:"received"`
	} `json:"data"`
}

type AdminStatsResponse struct {
	Success bool `json:"success"`
	Data    struct {
		ActiveConnections int    `json:"active_connections"`
		UptimeSeconds     int    `json:"uptime_seconds"`
		MemoryUsageMB     int    `json:"memory_usage_mb"`
		AdminClient       string `json:"admin_client"`
	} `json:"data"`
}

// runClientDemo demonstrates the mTLS client functionality
func runClientDemo(client *MTLSClient, logger *log.Logger) {
	fmt.Println("🚀 Blueprint mTLS Client Demo")
	fmt.Println("==============================")

	// Test 1: Health check (no client certificate required)
	fmt.Println("\n1. Testing health endpoint (no client cert required)...")
	var healthResp HealthResponse
	if err := client.GetJSON("/health", &healthResp); err != nil {
		fmt.Printf("   ❌ Health check failed: %v\n", err)
		logger.Error(err, "Health check failed")
	} else {
		fmt.Printf("   ✅ Health check passed\n")
		fmt.Printf("   Server: %s, Status: %s\n", healthResp.Data.Server, healthResp.Data.Status)
	}

	// Test 2: Secure endpoint (client certificate required)
	fmt.Println("\n2. Testing secure endpoint (client cert required)...")
	var secureResp SecureResponse
	if err := client.GetJSON("/secure", &secureResp); err != nil {
		fmt.Printf("   ❌ mTLS authentication failed: %v\n", err)
		logger.Error(err, "Secure endpoint failed")
	} else {
		fmt.Printf("   ✅ mTLS authentication successful\n")
		fmt.Printf("   Message: %s\n", secureResp.Data.Message)
		if clientInfo := secureResp.Data.ClientInfo; clientInfo != nil {
			if commonName, ok := clientInfo["common_name"].(string); ok {
				fmt.Printf("   Client: %s\n", commonName)
			}
		}
	}

	// Test 3: User profile API
	fmt.Println("\n3. Testing user profile API...")
	var profileResp UserProfileResponse
	if err := client.GetJSON("/api/v1/user/profile", &profileResp); err != nil {
		fmt.Printf("   ❌ User profile retrieval failed: %v\n", err)
		logger.Error(err, "User profile API failed")
	} else {
		fmt.Printf("   ✅ User profile retrieved successfully\n")
		fmt.Printf("   User: %s (%s)\n", profileResp.Data.Username, profileResp.Data.Email)
		fmt.Printf("   Privileges: %v\n", profileResp.Data.Privileges)
	}

	// Test 4: Data submission API
	fmt.Println("\n4. Testing data submission API...")
	testData := map[string]interface{}{
		"message":   "Hello from Blueprint mTLS client!",
		"timestamp": time.Now().Format(time.RFC3339),
		"client_info": map[string]interface{}{
			"version":     "1.0",
			"language":    "Go",
			"framework":   "Blueprint",
		},
		"metrics": map[string]interface{}{
			"cpu_usage":    42.5,
			"memory_usage": 128.7,
			"connections":  15,
		},
	}

	var dataResp DataResponse
	if err := client.PostJSON("/api/v1/data", testData, &dataResp); err != nil {
		fmt.Printf("   ❌ Data submission failed: %v\n", err)
		logger.Error(err, "Data submission failed")
	} else {
		fmt.Printf("   ✅ Data submitted successfully\n")
		fmt.Printf("   Data ID: %s\n", dataResp.Data.DataID)
	}

	// Test 5: Admin stats API
	fmt.Println("\n5. Testing admin stats API...")
	var adminResp AdminStatsResponse
	if err := client.GetJSON("/api/v1/admin/stats", &adminResp); err != nil {
		fmt.Printf("   ❌ Admin stats retrieval failed: %v\n", err)
		if strings.Contains(err.Error(), "403") {
			fmt.Printf("   ⚠️  Admin access denied (expected for demo client)\n")
		}
	} else {
		fmt.Printf("   ✅ Admin stats retrieved successfully\n")
		fmt.Printf("   Active connections: %d, Memory: %d MB\n", 
			adminResp.Data.ActiveConnections, adminResp.Data.MemoryUsageMB)
	}

	fmt.Println("\n✅ Blueprint mTLS Client Demo completed!")
}

// runPerformanceTest demonstrates client performance
func runPerformanceTest(client *MTLSClient, logger *log.Logger) {
	fmt.Println("\n🏃 Performance Test")
	fmt.Println("==================")

	// Warm up
	client.Get("/health")

	numRequests := 50
	start := time.Now()
	successCount := 0

	for i := 0; i < numRequests; i++ {
		resp, err := client.Get("/health")
		if err != nil {
			logger.Error(err, "Performance test request failed", log.KV{"request": i})
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == 200 {
			successCount++
		}
	}

	duration := time.Since(start)
	fmt.Printf("Completed %d/%d requests in %v\n", successCount, numRequests, duration)
	if successCount > 0 {
		fmt.Printf("Average request time: %v\n", duration/time.Duration(successCount))
		fmt.Printf("Requests per second: %.2f\n", float64(successCount)/duration.Seconds())
	} else {
		fmt.Printf("No successful requests - check if server is running\n")
	}
}

func main() {
	// Setup logger
	logger := log.New("mtls-client")
	logger.Info("Starting Blueprint mTLS client demo...")

	// Create client configuration
	config := &ClientConfig{
		ServerURL:  "https://localhost:8444",
		CACert:     "../certs/ca.crt",
		ClientCert: "../certs/client.crt",
		ClientKey:  "../certs/client.key",
		Timeout:    30 * time.Second,
	}

	// Create mTLS client
	client, err := NewMTLSClient(config, logger)
	if err != nil {
		logger.Fatal(err, "Failed to create mTLS client")
	}
	defer client.Close()

	// Check command line arguments
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "performance":
			runPerformanceTest(client, logger)
			return
		case "single":
			if len(os.Args) < 4 {
				fmt.Println("Usage: go run main.go single <method> <path> [json-body]")
				return
			}
			runSingleRequest(client, os.Args[2], os.Args[3], os.Args[4:])
			return
		}
	}

	// Run full demo by default
	runClientDemo(client, logger)
}

// runSingleRequest makes a single request for testing
func runSingleRequest(client *MTLSClient, method, path string, bodyArgs []string) {
	var body interface{}
	if len(bodyArgs) > 0 {
		if err := json.Unmarshal([]byte(bodyArgs[0]), &body); err != nil {
			fmt.Printf("Invalid JSON body: %v\n", err)
			return
		}
	}

	var resp *http.Response
	var err error

	switch strings.ToUpper(method) {
	case "GET":
		resp, err = client.Get(path)
	case "POST":
		resp, err = client.Post(path, body)
	case "PUT":
		resp, err = client.Put(path, body)
	case "DELETE":
		resp, err = client.Delete(path)
	default:
		fmt.Printf("Unsupported method: %s\n", method)
		return
	}

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Response: %s\n", string(responseBody))
}