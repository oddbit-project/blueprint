package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// mockResponse helps capture validation errors from response.ValidationError
type mockResponse struct {
	Success bool `json:"success"`
	Error   struct {
		Message      string            `json:"message"`
		RequestError []ValidationError `json:"requestError"`
	} `json:"error"`
}

// getErrors extracts validation errors from response
func (m *mockResponse) getErrors() []ValidationError {
	return m.Error.RequestError
}

// Test types for top-level custom validation
type CustomRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (c *CustomRequest) Validate() error {
	if c.Username == "admin" && len(c.Password) < 12 {
		return errors.New("admin password must be at least 12 characters")
	}
	return nil
}

// Test types for nested validation
type ValidatableAddress struct {
	Street  string `json:"street" binding:"required"`
	ZipCode string `json:"zip_code" binding:"required,len=5"`
}

func (a *ValidatableAddress) Validate() error {
	if a.ZipCode == "00000" {
		return errors.New("invalid zip code")
	}
	return nil
}

type UserRequest struct {
	Name    string             `json:"name" binding:"required"`
	Address ValidatableAddress `json:"address" binding:"required"`
}

// Test types for slice validation
type ValidatableItem struct {
	Name string `json:"name" binding:"required"`
}

func (i *ValidatableItem) Validate() error {
	if i.Name == "forbidden" {
		return errors.New("forbidden item name")
	}
	return nil
}

type OrderRequest struct {
	Items []ValidatableItem `json:"items" binding:"required,dive"`
}

// TestBasicBindingValidation ensures binding validation works as before
func TestBasicBindingValidation(t *testing.T) {
	type LoginRequest struct {
		Username string `json:"username" binding:"required,email"`
		Password string `json:"password" binding:"required,min=8"`
	}

	tests := []struct {
		name           string
		payload        string
		expectValid    bool
		expectedField  string
		expectedErrMsg string
	}{
		{
			name:        "valid request",
			payload:     `{"username":"test@example.com","password":"password123"}`,
			expectValid: true,
		},
		{
			name:           "missing required field",
			payload:        `{"username":"test@example.com"}`,
			expectValid:    false,
			expectedField:  "Password",
			expectedErrMsg: "Error: Field validation failed on the 'required' validator",
		},
		{
			name:           "invalid email format",
			payload:        `{"username":"notanemail","password":"password123"}`,
			expectValid:    false,
			expectedField:  "Username",
			expectedErrMsg: "Error: Field validation failed on the 'email' validator",
		},
		{
			name:           "password too short",
			payload:        `{"username":"test@example.com","password":"short"}`,
			expectValid:    false,
			expectedField:  "Password",
			expectedErrMsg: "Error: Field validation failed on the 'min' validator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString(tt.payload))
			c.Request.Header.Set("Content-Type", "application/json")

			var req LoginRequest
			result := ValidateJSON(c, &req)

			if result != tt.expectValid {
				t.Errorf("expected valid=%v, got %v", tt.expectValid, result)
			}

			if !tt.expectValid {
				var resp mockResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}

				errors := resp.getErrors()
				if len(errors) == 0 {
					t.Fatal("expected validation errors, got none")
				}

				found := false
				for _, err := range errors {
					if err.Field == tt.expectedField {
						found = true
						if err.Message != tt.expectedErrMsg {
							t.Errorf("expected message '%s', got '%s'", tt.expectedErrMsg, err.Message)
						}
						break
					}
				}

				if !found {
					t.Errorf("expected error for field '%s', but not found in: %+v", tt.expectedField, errors)
				}
			}
		})
	}
}

// TestTopLevelCustomValidation ensures top-level custom validation returns "field": "custom"
func TestTopLevelCustomValidation(t *testing.T) {
	tests := []struct {
		name           string
		payload        string
		expectValid    bool
		expectedField  string
		expectedErrMsg string
	}{
		{
			name:        "valid admin password",
			payload:     `{"username":"admin","password":"verylongpassword123"}`,
			expectValid: true,
		},
		{
			name:           "invalid admin password",
			payload:        `{"username":"admin","password":"short"}`,
			expectValid:    false,
			expectedField:  "custom",
			expectedErrMsg: "admin password must be at least 12 characters",
		},
		{
			name:        "non-admin user",
			payload:     `{"username":"user","password":"short"}`,
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString(tt.payload))
			c.Request.Header.Set("Content-Type", "application/json")

			var req CustomRequest
			result := ValidateJSON(c, &req)

			if result != tt.expectValid {
				t.Errorf("expected valid=%v, got %v", tt.expectValid, result)
			}

			if !tt.expectValid {
				var resp mockResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}

				errors := resp.getErrors()
				if len(errors) == 0 {
					t.Fatal("expected validation errors, got none")
				}

				if errors[0].Field != tt.expectedField {
					t.Errorf("COMPATIBILITY BREAK: expected field '%s', got '%s'", tt.expectedField, errors[0].Field)
				}

				if errors[0].Message != tt.expectedErrMsg {
					t.Errorf("COMPATIBILITY BREAK: expected message '%s', got '%s'", tt.expectedErrMsg, errors[0].Message)
				}
			}
		})
	}
}

// TestNestedStructValidation ensures nested validation includes field paths
func TestNestedStructValidation(t *testing.T) {
	tests := []struct {
		name           string
		payload        string
		expectValid    bool
		expectedField  string
		expectedErrMsg string
	}{
		{
			name:        "valid nested struct",
			payload:     `{"name":"John","address":{"street":"Main St","zip_code":"12345"}}`,
			expectValid: true,
		},
		{
			name:           "nested validation error",
			payload:        `{"name":"John","address":{"street":"Main St","zip_code":"00000"}}`,
			expectValid:    false,
			expectedField:  "address",
			expectedErrMsg: "invalid zip code",
		},
		{
			name:           "nested binding error",
			payload:        `{"name":"John","address":{"street":"Main St","zip_code":"123"}}`,
			expectValid:    false,
			expectedField:  "ZipCode",
			expectedErrMsg: "Error: Field validation failed on the 'len' validator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString(tt.payload))
			c.Request.Header.Set("Content-Type", "application/json")

			var req UserRequest
			result := ValidateJSON(c, &req)

			if result != tt.expectValid {
				t.Errorf("expected valid=%v, got %v", tt.expectValid, result)
			}

			if !tt.expectValid {
				var resp mockResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}

				errors := resp.getErrors()
				if len(errors) == 0 {
					t.Fatal("expected validation errors, got none")
				}

				if errors[0].Field != tt.expectedField {
					t.Errorf("expected field '%s', got '%s'", tt.expectedField, errors[0].Field)
				}

				if errors[0].Message != tt.expectedErrMsg {
					t.Errorf("expected message '%s', got '%s'", tt.expectedErrMsg, errors[0].Message)
				}
			}
		})
	}
}

// TestTimeValidation ensures time.Time fields work correctly
func TestTimeValidation(t *testing.T) {
	type EventRequest struct {
		Name      string    `json:"name" binding:"required"`
		StartTime time.Time `json:"start_time" binding:"required"`
	}

	tests := []struct {
		name        string
		payload     string
		expectValid bool
	}{
		{
			name:        "valid RFC3339 time",
			payload:     `{"name":"Event","start_time":"2023-10-15T10:00:00Z"}`,
			expectValid: true,
		},
		{
			name:        "invalid time format",
			payload:     `{"name":"Event","start_time":"not-a-time"}`,
			expectValid: false,
		},
		{
			name:        "missing time field",
			payload:     `{"name":"Event"}`,
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString(tt.payload))
			c.Request.Header.Set("Content-Type", "application/json")

			var req EventRequest
			result := ValidateJSON(c, &req)

			if result != tt.expectValid {
				t.Errorf("expected valid=%v, got %v (body: %s)", tt.expectValid, result, w.Body.String())
			}
		})
	}
}

// TestJSONUnmarshalError ensures JSON parse errors are handled properly
func TestJSONUnmarshalError(t *testing.T) {
	type SimpleRequest struct {
		Name string `json:"name" binding:"required"`
	}

	tests := []struct {
		name        string
		payload     string
		expectValid bool
	}{
		{
			name:        "invalid JSON",
			payload:     `{"name": invalid}`,
			expectValid: false,
		},
		{
			name:        "malformed JSON",
			payload:     `{name: "test"`,
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString(tt.payload))
			c.Request.Header.Set("Content-Type", "application/json")

			var req SimpleRequest
			result := ValidateJSON(c, &req)

			if result != tt.expectValid {
				t.Errorf("expected valid=%v, got %v", tt.expectValid, result)
			}

			if !tt.expectValid {
				var resp mockResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}

				errors := resp.getErrors()
				if len(errors) == 0 {
					t.Fatal("expected validation errors, got none")
				}

				// JSON unmarshal errors should have field "body"
				if errors[0].Field != "body" {
					t.Errorf("expected field 'body', got '%s'", errors[0].Field)
				}
			}
		})
	}
}

// TestSliceValidation ensures slice elements are validated
func TestSliceValidation(t *testing.T) {
	tests := []struct {
		name          string
		payload       string
		expectValid   bool
		expectedField string
	}{
		{
			name:        "valid items",
			payload:     `{"items":[{"name":"item1"},{"name":"item2"}]}`,
			expectValid: true,
		},
		{
			name:          "forbidden item in slice",
			payload:       `{"items":[{"name":"item1"},{"name":"forbidden"}]}`,
			expectValid:   false,
			expectedField: "items[1]",
		},
		{
			name:          "missing required field in slice",
			payload:       `{"items":[{"name":"item1"},{}]}`,
			expectValid:   false,
			expectedField: "Name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString(tt.payload))
			c.Request.Header.Set("Content-Type", "application/json")

			var req OrderRequest
			result := ValidateJSON(c, &req)

			if result != tt.expectValid {
				t.Errorf("expected valid=%v, got %v", tt.expectValid, result)
			}

			if !tt.expectValid && tt.expectedField != "" {
				var resp mockResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}

				errors := resp.getErrors()
				if len(errors) == 0 {
					t.Fatal("expected validation errors, got none")
				}

				if errors[0].Field != tt.expectedField {
					t.Errorf("expected field '%s', got '%s'", tt.expectedField, errors[0].Field)
				}
			}
		})
	}
}

// Test type for backward compatibility
type CompatRequest struct {
	Email string `json:"email" binding:"required,email"`
}

func (c *CompatRequest) Validate() error {
	if c.Email == "blocked@example.com" {
		return errors.New("this email is blocked")
	}
	return nil
}

// Test types for NewFieldError
type PasswordRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (p *PasswordRequest) Validate() error {
	if p.Username == "admin" && len(p.Password) < 12 {
		return NewFieldError("password", "admin password must be at least 12 characters")
	}
	return nil
}

type AccountRequest struct {
	Name     string          `json:"name" binding:"required"`
	Password PasswordRequest `json:"password" binding:"required"`
}

// TestNewFieldError tests the NewFieldError helper function
func TestNewFieldError(t *testing.T) {
	tests := []struct {
		name           string
		payload        string
		expectValid    bool
		expectedField  string
		expectedErrMsg string
	}{
		{
			name:           "top-level NewFieldError",
			payload:        `{"username":"admin","password":"short"}`,
			expectValid:    false,
			expectedField:  "password",
			expectedErrMsg: "admin password must be at least 12 characters",
		},
		{
			name:           "nested NewFieldError",
			payload:        `{"name":"Test","password":{"username":"admin","password":"short"}}`,
			expectValid:    false,
			expectedField:  "password.password",
			expectedErrMsg: "admin password must be at least 12 characters",
		},
		{
			name:        "valid request",
			payload:     `{"username":"admin","password":"verylongpassword"}`,
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString(tt.payload))
			c.Request.Header.Set("Content-Type", "application/json")

			var result bool
			if tt.name == "nested NewFieldError" {
				var req AccountRequest
				result = ValidateJSON(c, &req)
			} else {
				var req PasswordRequest
				result = ValidateJSON(c, &req)
			}

			if result != tt.expectValid {
				t.Errorf("expected valid=%v, got %v", tt.expectValid, result)
			}

			if !tt.expectValid {
				var resp mockResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}

				errors := resp.getErrors()
				if len(errors) == 0 {
					t.Fatal("expected validation errors, got none")
				}

				if errors[0].Field != tt.expectedField {
					t.Errorf("expected field '%s', got '%s'", tt.expectedField, errors[0].Field)
				}

				if errors[0].Message != tt.expectedErrMsg {
					t.Errorf("expected message '%s', got '%s'", tt.expectedErrMsg, errors[0].Message)
				}
			}
		})
	}
}

// Test types for map validation
// Note: For map validation to work with pointer receiver methods,
// use pointer values: map[string]*Type
type MapConfig struct {
	Value string `json:"value" binding:"required"`
}

func (c *MapConfig) Validate() error {
	if c.Value == "forbidden" {
		return errors.New("forbidden config value")
	}
	return nil
}

type MapSettings struct {
	Configs map[string]*MapConfig `json:"configs" binding:"required"`
}

// TestMapValidation ensures map values are validated
// Note: Map values must be pointers for pointer receiver Validate() methods to work
func TestMapValidation(t *testing.T) {

	tests := []struct {
		name          string
		payload       string
		expectValid   bool
		expectedField string
	}{
		{
			name:        "valid map values",
			payload:     `{"configs":{"key1":{"value":"ok"},"key2":{"value":"good"}}}`,
			expectValid: true,
		},
		{
			name:          "invalid map value",
			payload:       `{"configs":{"key1":{"value":"ok"},"key2":{"value":"forbidden"}}}`,
			expectValid:   false,
			expectedField: "configs[key2]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString(tt.payload))
			c.Request.Header.Set("Content-Type", "application/json")

			var req MapSettings
			result := ValidateJSON(c, &req)

			if result != tt.expectValid {
				t.Errorf("expected valid=%v, got %v", tt.expectValid, result)
			}

			if !tt.expectValid && tt.expectedField != "" {
				var resp mockResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}

				errors := resp.getErrors()
				if len(errors) == 0 {
					t.Fatal("expected validation errors, got none")
				}

				if errors[0].Field != tt.expectedField {
					t.Errorf("expected field '%s', got '%s'", tt.expectedField, errors[0].Field)
				}
			}
		})
	}
}

// Test types for pointer validation
type OptionalData struct {
	Value string `json:"value" binding:"required"`
}

func (o *OptionalData) Validate() error {
	if o.Value == "invalid" {
		return NewFieldError("value", "value cannot be 'invalid'")
	}
	return nil
}

type RequestWithPointer struct {
	Name string        `json:"name" binding:"required"`
	Data *OptionalData `json:"data"`
}

// TestPointerFieldValidation ensures pointer fields are handled correctly
func TestPointerFieldValidation(t *testing.T) {

	tests := []struct {
		name          string
		payload       string
		expectValid   bool
		expectedField string
	}{
		{
			name:        "nil pointer field",
			payload:     `{"name":"test"}`,
			expectValid: true,
		},
		{
			name:        "valid pointer field",
			payload:     `{"name":"test","data":{"value":"ok"}}`,
			expectValid: true,
		},
		{
			name:          "invalid pointer field",
			payload:       `{"name":"test","data":{"value":"invalid"}}`,
			expectValid:   false,
			expectedField: "data.value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString(tt.payload))
			c.Request.Header.Set("Content-Type", "application/json")

			var req RequestWithPointer
			result := ValidateJSON(c, &req)

			if result != tt.expectValid {
				t.Errorf("expected valid=%v, got %v", tt.expectValid, result)
			}

			if !tt.expectValid {
				var resp mockResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}

				errors := resp.getErrors()
				if len(errors) == 0 {
					t.Fatal("expected validation errors, got none")
				}

				if errors[0].Field != tt.expectedField {
					t.Errorf("expected field '%s', got '%s'", tt.expectedField, errors[0].Field)
				}
			}
		})
	}
}

// Test types for deep nesting
type Level3 struct {
	Value string `json:"value" binding:"required"`
}

func (l Level3) Validate() error {
	if l.Value == "bad" {
		return NewFieldError("value", "cannot be 'bad'")
	}
	return nil
}

type Level2 struct {
	Level3 Level3 `json:"level3" binding:"required"`
}

type Level1 struct {
	Level2 Level2 `json:"level2" binding:"required"`
}

// TestDeepNestedValidation ensures deeply nested structures work
func TestDeepNestedValidation(t *testing.T) {

	tests := []struct {
		name          string
		payload       string
		expectValid   bool
		expectedField string
	}{
		{
			name:        "valid deep nesting",
			payload:     `{"level2":{"level3":{"value":"ok"}}}`,
			expectValid: true,
		},
		{
			name:          "invalid deep nesting",
			payload:       `{"level2":{"level3":{"value":"bad"}}}`,
			expectValid:   false,
			expectedField: "level2.level3.value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString(tt.payload))
			c.Request.Header.Set("Content-Type", "application/json")

			var req Level1
			result := ValidateJSON(c, &req)

			if result != tt.expectValid {
				t.Errorf("expected valid=%v, got %v", tt.expectValid, result)
			}

			if !tt.expectValid {
				var resp mockResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}

				errors := resp.getErrors()
				if len(errors) == 0 {
					t.Fatal("expected validation errors, got none")
				}

				if errors[0].Field != tt.expectedField {
					t.Errorf("expected field '%s', got '%s'", tt.expectedField, errors[0].Field)
				}
			}
		})
	}
}

// Test types for array validation
type FixedItem struct {
	Name string `json:"name" binding:"required"`
}

func (f FixedItem) Validate() error {
	if f.Name == "invalid" {
		return errors.New("invalid item name")
	}
	return nil
}

type ArrayRequest struct {
	Items [3]FixedItem `json:"items" binding:"required,dive"`
}

// TestArrayValidation ensures array (not slice) validation works
func TestArrayValidation(t *testing.T) {

	tests := []struct {
		name          string
		payload       string
		expectValid   bool
		expectedField string
	}{
		{
			name:        "valid array",
			payload:     `{"items":[{"name":"a"},{"name":"b"},{"name":"c"}]}`,
			expectValid: true,
		},
		{
			name:          "invalid array element",
			payload:       `{"items":[{"name":"a"},{"name":"invalid"},{"name":"c"}]}`,
			expectValid:   false,
			expectedField: "items[1]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString(tt.payload))
			c.Request.Header.Set("Content-Type", "application/json")

			var req ArrayRequest
			result := ValidateJSON(c, &req)

			if result != tt.expectValid {
				t.Errorf("expected valid=%v, got %v", tt.expectValid, result)
			}

			if !tt.expectValid {
				var resp mockResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}

				errors := resp.getErrors()
				if len(errors) == 0 {
					t.Fatal("expected validation errors, got none")
				}

				if errors[0].Field != tt.expectedField {
					t.Errorf("expected field '%s', got '%s'", tt.expectedField, errors[0].Field)
				}
			}
		})
	}
}

// Test types for cross-field validation
type DateRange struct {
	StartDate string `json:"start_date" binding:"required"`
	EndDate   string `json:"end_date" binding:"required"`
}

func (d *DateRange) Validate() error {
	if d.StartDate > d.EndDate {
		return errors.New("start_date must be before end_date")
	}
	return nil
}

// TestCrossFieldValidation demonstrates cross-field validation
func TestCrossFieldValidation(t *testing.T) {

	tests := []struct {
		name          string
		payload       string
		expectValid   bool
		expectedField string
	}{
		{
			name:        "valid date range",
			payload:     `{"start_date":"2024-01-01","end_date":"2024-12-31"}`,
			expectValid: true,
		},
		{
			name:          "invalid date range",
			payload:       `{"start_date":"2024-12-31","end_date":"2024-01-01"}`,
			expectValid:   false,
			expectedField: "custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString(tt.payload))
			c.Request.Header.Set("Content-Type", "application/json")

			var req DateRange
			result := ValidateJSON(c, &req)

			if result != tt.expectValid {
				t.Errorf("expected valid=%v, got %v", tt.expectValid, result)
			}

			if !tt.expectValid {
				var resp mockResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}

				errors := resp.getErrors()
				if len(errors) == 0 {
					t.Fatal("expected validation errors, got none")
				}

				if errors[0].Field != tt.expectedField {
					t.Errorf("expected field '%s', got '%s'", tt.expectedField, errors[0].Field)
				}
			}
		})
	}
}

// TestEmptyStruct ensures empty structs don't cause issues
func TestEmptyStruct(t *testing.T) {
	type EmptyRequest struct{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	var req EmptyRequest
	result := ValidateJSON(c, &req)

	if !result {
		t.Errorf("expected empty struct to be valid")
	}
}

// TestUnexportedFields ensures unexported fields are skipped
func TestUnexportedFields(t *testing.T) {
	type WithUnexported struct {
		Public  string `json:"public" binding:"required"`
		private string
	}

	tests := []struct {
		name        string
		payload     string
		expectValid bool
	}{
		{
			name:        "valid with exported field",
			payload:     `{"public":"value"}`,
			expectValid: true,
		},
		{
			name:        "unexported field ignored",
			payload:     `{"public":"value","private":"ignored"}`,
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString(tt.payload))
			c.Request.Header.Set("Content-Type", "application/json")

			var req WithUnexported
			result := ValidateJSON(c, &req)

			if result != tt.expectValid {
				t.Errorf("expected valid=%v, got %v", tt.expectValid, result)
			}
		})
	}
}

// TestBackwardCompatibility ensures all error formats match original implementation
func TestBackwardCompatibility(t *testing.T) {
	tests := []struct {
		name            string
		payload         string
		expectedField   string
		expectedMessage string
	}{
		{
			name:            "binding validation error format",
			payload:         `{"email":"notanemail"}`,
			expectedField:   "Email",
			expectedMessage: "Error: Field validation failed on the 'email' validator",
		},
		{
			name:            "custom validation error format",
			payload:         `{"email":"blocked@example.com"}`,
			expectedField:   "custom",
			expectedMessage: "this email is blocked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString(tt.payload))
			c.Request.Header.Set("Content-Type", "application/json")

			var req CompatRequest
			ValidateJSON(c, &req)

			var resp mockResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}

			errors := resp.getErrors()
			if len(errors) == 0 {
				t.Fatal("expected validation errors, got none")
			}

			if errors[0].Field != tt.expectedField {
				t.Errorf("COMPATIBILITY BREAK: expected field '%s', got '%s'", tt.expectedField, errors[0].Field)
			}

			if errors[0].Message != tt.expectedMessage {
				t.Errorf("COMPATIBILITY BREAK: expected message '%s', got '%s'", tt.expectedMessage, errors[0].Message)
			}
		})
	}
}

// Query validation test types
type SearchRequest struct {
	Query    string `form:"q" binding:"required,min=3"`
	Page     int    `form:"page" binding:"min=1"`
	PageSize int    `form:"page_size" binding:"min=10,max=100"`
}

func (s *SearchRequest) Validate() error {
	if s.Page > 1000 {
		return NewFieldError("page", "maximum page number is 1000")
	}
	return nil
}

type FilterRequest struct {
	Category string `form:"category" binding:"required"`
	MinPrice int    `form:"min_price" binding:"min=0"`
	MaxPrice int    `form:"max_price" binding:"min=0"`
}

func (f *FilterRequest) Validate() error {
	if f.MaxPrice > 0 && f.MinPrice > f.MaxPrice {
		return errors.New("min_price cannot be greater than max_price")
	}
	return nil
}

// TestBasicQueryValidation ensures basic query parameter validation works
func TestBasicQueryValidation(t *testing.T) {
	tests := []struct {
		name           string
		queryString    string
		expectValid    bool
		expectedField  string
		expectedErrMsg string
	}{
		{
			name:        "valid query parameters",
			queryString: "?q=test&page=1&page_size=20",
			expectValid: true,
		},
		{
			name:           "missing required parameter",
			queryString:    "?page=1&page_size=20",
			expectValid:    false,
			expectedField:  "Query",
			expectedErrMsg: "Error: Field validation failed on the 'required' validator",
		},
		{
			name:           "query too short",
			queryString:    "?q=ab&page=1&page_size=20",
			expectValid:    false,
			expectedField:  "Query",
			expectedErrMsg: "Error: Field validation failed on the 'min' validator",
		},
		{
			name:           "page too small",
			queryString:    "?q=test&page=0&page_size=20",
			expectValid:    false,
			expectedField:  "Page",
			expectedErrMsg: "Error: Field validation failed on the 'min' validator",
		},
		{
			name:           "page_size too small",
			queryString:    "?q=test&page=1&page_size=5",
			expectValid:    false,
			expectedField:  "PageSize",
			expectedErrMsg: "Error: Field validation failed on the 'min' validator",
		},
		{
			name:           "page_size too large",
			queryString:    "?q=test&page=1&page_size=200",
			expectValid:    false,
			expectedField:  "PageSize",
			expectedErrMsg: "Error: Field validation failed on the 'max' validator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/search"+tt.queryString, nil)
			c.Request.Header.Set("Content-Type", "application/json")

			var req SearchRequest
			result := ValidateQuery(c, &req)

			if result != tt.expectValid {
				t.Errorf("expected valid=%v, got %v", tt.expectValid, result)
			}

			if !tt.expectValid {
				var resp mockResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}

				errors := resp.getErrors()
				if len(errors) == 0 {
					t.Fatal("expected validation errors, got none")
				}

				found := false
				for _, err := range errors {
					if err.Field == tt.expectedField {
						found = true
						if err.Message != tt.expectedErrMsg {
							t.Errorf("expected message '%s', got '%s'", tt.expectedErrMsg, err.Message)
						}
						break
					}
				}

				if !found {
					t.Errorf("expected error for field '%s', but not found in: %+v", tt.expectedField, errors)
				}
			}
		})
	}
}

// TestQueryCustomValidation ensures custom validation works for query parameters
func TestQueryCustomValidation(t *testing.T) {
	tests := []struct {
		name           string
		queryString    string
		expectValid    bool
		expectedField  string
		expectedErrMsg string
	}{
		{
			name:        "valid page number",
			queryString: "?q=test&page=100&page_size=20",
			expectValid: true,
		},
		{
			name:           "page number too high",
			queryString:    "?q=test&page=1001&page_size=20",
			expectValid:    false,
			expectedField:  "page",
			expectedErrMsg: "maximum page number is 1000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/search"+tt.queryString, nil)
			c.Request.Header.Set("Content-Type", "application/json")

			var req SearchRequest
			result := ValidateQuery(c, &req)

			if result != tt.expectValid {
				t.Errorf("expected valid=%v, got %v", tt.expectValid, result)
			}

			if !tt.expectValid {
				var resp mockResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}

				errors := resp.getErrors()
				if len(errors) == 0 {
					t.Fatal("expected validation errors, got none")
				}

				if errors[0].Field != tt.expectedField {
					t.Errorf("expected field '%s', got '%s'", tt.expectedField, errors[0].Field)
				}

				if errors[0].Message != tt.expectedErrMsg {
					t.Errorf("expected message '%s', got '%s'", tt.expectedErrMsg, errors[0].Message)
				}
			}
		})
	}
}

// TestQueryCrossFieldValidation ensures cross-field validation works for query parameters
func TestQueryCrossFieldValidation(t *testing.T) {
	tests := []struct {
		name           string
		queryString    string
		expectValid    bool
		expectedField  string
		expectedErrMsg string
	}{
		{
			name:        "valid price range",
			queryString: "?category=books&min_price=10&max_price=100",
			expectValid: true,
		},
		{
			name:        "no max price",
			queryString: "?category=books&min_price=10",
			expectValid: true,
		},
		{
			name:           "invalid price range",
			queryString:    "?category=books&min_price=100&max_price=10",
			expectValid:    false,
			expectedField:  "custom",
			expectedErrMsg: "min_price cannot be greater than max_price",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/filter"+tt.queryString, nil)
			c.Request.Header.Set("Content-Type", "application/json")

			var req FilterRequest
			result := ValidateQuery(c, &req)

			if result != tt.expectValid {
				t.Errorf("expected valid=%v, got %v", tt.expectValid, result)
			}

			if !tt.expectValid {
				var resp mockResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}

				errors := resp.getErrors()
				if len(errors) == 0 {
					t.Fatal("expected validation errors, got none")
				}

				if errors[0].Field != tt.expectedField {
					t.Errorf("expected field '%s', got '%s'", tt.expectedField, errors[0].Field)
				}

				if errors[0].Message != tt.expectedErrMsg {
					t.Errorf("expected message '%s', got '%s'", tt.expectedErrMsg, errors[0].Message)
				}
			}
		})
	}
}

// TestQueryTypeConversionError ensures type conversion errors are handled
func TestQueryTypeConversionError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/search?q=test&page=invalid&page_size=20", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	var req SearchRequest
	result := ValidateQuery(c, &req)

	if result {
		t.Error("expected validation to fail for invalid type conversion")
	}

	var resp mockResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	errors := resp.getErrors()
	if len(errors) == 0 {
		t.Fatal("expected validation errors, got none")
	}

	// Type conversion errors should have field "-" for backward compatibility
	if errors[0].Field != "-" {
		t.Errorf("expected field '-', got '%s'", errors[0].Field)
	}
}
