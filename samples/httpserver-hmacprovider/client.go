package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/oddbit-project/blueprint/provider/httpserver/auth"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/provider/hmacprovider"
)

const KeyId = "myKey"

// HMACClient demonstrates how to make authenticated requests to the HMAC server
type HMACClient struct {
	baseURL  string
	provider *hmacprovider.HMACProvider
	client   *http.Client
	keyId    string
}

// NewHMACClient creates a new HMAC client
func NewHMACClient(baseURL string, keyId string, hmacSecret string) (*HMACClient, error) {
	// Generate encryption key
	key, err := secure.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	// Create secret (must match server secret)
	secret, err := secure.NewCredential([]byte(hmacSecret), key, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create credential: %w", err)
	}

	// create key provider
	keyProvider := hmacprovider.NewSingleKeyProvider(keyId, secret)

	// Create HMAC provider (same config as server)
	provider := hmacprovider.NewHmacProvider(
		keyProvider,
		hmacprovider.WithKeyInterval(5*time.Minute),
		hmacprovider.WithMaxInputSize(10*1024*1024),
	)

	return &HMACClient{
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		provider: provider,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		keyId: keyId,
	}, nil
}

// makeRequest creates and sends an authenticated HTTP request
func (c *HMACClient) makeRequest(method, path string, body interface{}) (*http.Response, error) {
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

	// Set content type
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Generate HMAC signature
	bodyReader := bytes.NewReader(requestBody)
	hash, timestamp, nonce, err := c.provider.Sign256(c.keyId, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	// Add HMAC headers
	req.Header.Set(auth.HeaderHMACHash, hash)
	req.Header.Set(auth.HeaderHMACTimestamp, timestamp)
	req.Header.Set(auth.HeaderHMACNonce, nonce)

	// Make request
	return c.client.Do(req)
}

// Get request
func (c *HMACClient) Get(path string) (*http.Response, error) {
	return c.makeRequest("GET", path, nil)
}

// Post request
func (c *HMACClient) Post(path string, body interface{}) (*http.Response, error) {
	return c.makeRequest("POST", path, body)
}

// Put request
func (c *HMACClient) Put(path string, body interface{}) (*http.Response, error) {
	return c.makeRequest("PUT", path, body)
}

// Delete request
func (c *HMACClient) Delete(path string) (*http.Response, error) {
	return c.makeRequest("DELETE", path, nil)
}

// Example client usage
func runClientExamples() {
	fmt.Println("=== HMAC Client Examples ===")

	// Create client (must use same secret as server)
	client, err := NewHMACClient("http://localhost:8080", KeyId, "your-hmac-secret-key-change-this-in-production")
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		return
	}

	// Example 1: Test public endpoint (no authentication required)
	fmt.Println("\n1. Testing public endpoint (GET /api/public/health)")
	resp, err := http.Get("http://localhost:8080/api/public/health")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Status: %d\nResponse: %s\n", resp.StatusCode, string(body))
		resp.Body.Close()
	}

	// Example 2: Test protected endpoint (authentication required)
	fmt.Println("\n2. Testing protected endpoint (GET /api/protected/profile)")
	resp, err = client.Get("/api/protected/profile")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Status: %d\nResponse: %s\n", resp.StatusCode, string(body))
		resp.Body.Close()
	}

	// Example 3: Test POST with data
	fmt.Println("\n3. Testing POST with data (POST /api/protected/data)")
	postData := map[string]interface{}{
		"message": "Hello from HMAC client!",
		"type":    "greeting",
	}
	resp, err = client.Post("/api/protected/data", postData)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Status: %d\nResponse: %s\n", resp.StatusCode, string(body))
		resp.Body.Close()
	}

	// Example 4: Test PUT request
	fmt.Println("\n4. Testing PUT request (PUT /api/protected/settings)")
	settingsData := map[string]interface{}{
		"theme":    "dark",
		"language": "en",
		"timezone": "UTC",
		"preferences": map[string]interface{}{
			"notifications": true,
			"auto_save":     false,
		},
	}
	resp, err = client.Put("/api/protected/settings", settingsData)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Status: %d\nResponse: %s\n", resp.StatusCode, string(body))
		resp.Body.Close()
	}

	// Example 5: Test DELETE request
	fmt.Println("\n5. Testing DELETE request (DELETE /api/protected/resource/123)")
	resp, err = client.Delete("/api/protected/resource/123")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Status: %d\nResponse: %s\n", resp.StatusCode, string(body))
		resp.Body.Close()
	}

	// Example 6: Test admin endpoint
	fmt.Println("\n6. Testing admin endpoint (GET /api/admin/stats)")
	resp, err = client.Get("/api/admin/stats")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Status: %d\nResponse: %s\n", resp.StatusCode, string(body))
		resp.Body.Close()
	}

	// Example 7: Test sign endpoint (public)
	fmt.Println("\n7. Testing sign endpoint (POST /api/public/sign)")
	signData := map[string]string{
		"data": "This is test data to be signed",
	}
	signBody, _ := json.Marshal(signData)
	resp, err = http.Post("http://localhost:8080/api/public/sign", "application/json", bytes.NewReader(signBody))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Status: %d\nResponse: %s\n", resp.StatusCode, string(body))
		resp.Body.Close()
	}

	// Example 8: Demonstrate authentication failure with wrong secret
	fmt.Println("\n8. Testing authentication failure (wrong secret)")
	wrongClient, err := NewHMACClient("http://localhost:8080", KeyId, "wrong-secret")
	if err != nil {
		fmt.Printf("Failed to create client with wrong secret: %v\n", err)
		return
	}
	resp, err = wrongClient.Get("/api/protected/profile")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Status: %d (Expected 401)\nResponse: %s\n", resp.StatusCode, string(body))
		resp.Body.Close()
	}

	// Example 9: Demonstrate request without authentication headers
	fmt.Println("\n9. Testing request without authentication headers")
	req, _ := http.NewRequest("GET", "http://localhost:8080/api/protected/profile", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Status: %d (Expected 401)\nResponse: %s\n", resp.StatusCode, string(body))
		resp.Body.Close()
	}

	fmt.Println("\n=== Client Examples Complete ===")
}

// Performance test example
func runPerformanceTest() {
	fmt.Println("\n=== Performance Test ===")

	client, err := NewHMACClient("http://localhost:8080", KeyId, "your-hmac-secret-key-change-this-in-production")
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		return
	}

	// Warm up
	client.Get("/api/protected/profile")

	// Performance test
	numRequests := 100
	start := time.Now()

	for i := 0; i < numRequests; i++ {
		resp, err := client.Get("/api/protected/profile")
		if err != nil {
			fmt.Printf("Request %d failed: %v\n", i, err)
			continue
		}
		resp.Body.Close()
	}

	duration := time.Since(start)
	fmt.Printf("Completed %d requests in %v\n", numRequests, duration)
	fmt.Printf("Average request time: %v\n", duration/time.Duration(numRequests))
	fmt.Printf("Requests per second: %.2f\n", float64(numRequests)/duration.Seconds())
}

// Command-line interface for the client
func main() {
	if len(os.Args) < 2 {
		fmt.Println("HMAC Client Tool")
		fmt.Println("Usage:")
		fmt.Println("  go run client.go examples    - Run client examples")
		fmt.Println("  go run client.go performance - Run performance test")
		fmt.Println("  go run client.go request <method> <path> [json-body] - Make single request")
		return
	}

	command := os.Args[1]
	switch command {
	case "examples":
		runClientExamples()
	case "performance":
		runPerformanceTest()
	case "request":
		if len(os.Args) < 4 {
			fmt.Println("Usage: go run client.go request <method> <path> [json-body]")
			return
		}
		runSingleRequest(os.Args[2], os.Args[3], os.Args[4:])
	default:
		fmt.Printf("Unknown command: %s\n", command)
	}
}

// runSingleRequest makes a single authenticated request
func runSingleRequest(method, path string, bodyArgs []string) {
	client, err := NewHMACClient("http://localhost:8080", KeyId, "your-hmac-secret-key-change-this-in-production")
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		return
	}

	var body interface{}
	if len(bodyArgs) > 0 {
		if err := json.Unmarshal([]byte(bodyArgs[0]), &body); err != nil {
			fmt.Printf("Invalid JSON body: %v\n", err)
			return
		}
	}

	var resp *http.Response
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
