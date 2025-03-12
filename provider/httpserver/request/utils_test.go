package request

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestContext(t *testing.T, headers map[string]string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	
	req, err := http.NewRequest("GET", "/test", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	
	// Set headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	
	c.Request = req
	return c
}

func TestIsJSONRequest_WithAcceptHeader(t *testing.T) {
	// Test with Accept: application/json header
	ctx := setupTestContext(t, map[string]string{
		HeaderAccept: ContentTypeJson,
	})
	
	assert.True(t, IsJSONRequest(ctx))
}

func TestIsJSONRequest_WithContentTypeHeader(t *testing.T) {
	// Test with Content-Type: application/json header
	ctx := setupTestContext(t, map[string]string{
		HeaderContentType: ContentTypeJson,
	})
	
	assert.True(t, IsJSONRequest(ctx))
}

func TestIsJSONRequest_WithBothHeaders(t *testing.T) {
	// Test with both headers set to application/json
	ctx := setupTestContext(t, map[string]string{
		HeaderAccept:      ContentTypeJson,
		HeaderContentType: ContentTypeJson,
	})
	
	assert.True(t, IsJSONRequest(ctx))
}

func TestIsJSONRequest_WithNoHeaders(t *testing.T) {
	// Test with no relevant headers
	ctx := setupTestContext(t, map[string]string{})
	
	assert.False(t, IsJSONRequest(ctx))
}

func TestIsJSONRequest_WithDifferentHeaders(t *testing.T) {
	// Test with non-JSON headers
	ctx := setupTestContext(t, map[string]string{
		HeaderAccept:      ContentTypeHtml,
		HeaderContentType: ContentTypeBinary,
	})
	
	assert.False(t, IsJSONRequest(ctx))
}

func TestIsJSONRequest_WithMixedHeaders(t *testing.T) {
	// Test with mixed headers (one JSON, one not)
	ctx := setupTestContext(t, map[string]string{
		HeaderAccept:      ContentTypeHtml,
		HeaderContentType: ContentTypeJson,
	})
	
	assert.True(t, IsJSONRequest(ctx))
	
	ctx = setupTestContext(t, map[string]string{
		HeaderAccept:      ContentTypeJson,
		HeaderContentType: ContentTypeHtml,
	})
	
	assert.True(t, IsJSONRequest(ctx))
}

func TestIsJSONRequest_WithCaseSensitivity(t *testing.T) {
	// Test case sensitivity
	ctx := setupTestContext(t, map[string]string{
		HeaderAccept: "Application/JSON",
	})
	
	// This should return false because the comparison is case-sensitive
	assert.False(t, IsJSONRequest(ctx))
}

func TestContentTypeConstants(t *testing.T) {
	// Test that constants are defined correctly
	assert.Equal(t, "text/html", ContentTypeHtml)
	assert.Equal(t, "application/json", ContentTypeJson)
	assert.Equal(t, "application/octet-stream", ContentTypeBinary)
	
	assert.Equal(t, "Accept", HeaderAccept)
	assert.Equal(t, "Content-Type", HeaderContentType)
}