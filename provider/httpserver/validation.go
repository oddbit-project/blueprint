package httpserver

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	cv "github.com/oddbit-project/blueprint/provider/httpserver/request/validator"
	"github.com/oddbit-project/blueprint/provider/httpserver/response"
)

const (
	fieldErrMsg = "Error: Field validation failed on the '%s' validator"
)

// ValidationError represents a field validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Validator interface for custom validation logic
// Implement this interface on your request structs to add custom validation
// that runs after binding validation succeeds
type Validator interface {
	Validate() error
}

// FieldError wraps validation errors with field path context
type FieldError struct {
	Field string
	Err   error
}

func (e *FieldError) Error() string {
	return e.Err.Error()
}

func (e *FieldError) Unwrap() error {
	return e.Err
}

// NewFieldError creates a custom validation error for a specific field
// Use this in your Validate() method to return field-specific errors
//
// Example:
//
//	func (r *LoginRequest) Validate() error {
//	    if r.Username == "admin" && len(r.Password) < 12 {
//	        return NewFieldError("password", "admin password must be at least 12 characters")
//	    }
//	    return nil
//	}
//
// This will produce: {"field": "password", "message": "admin password must be at least 12 characters"}
// For nested structs: {"field": "address.password", "message": "..."}
func NewFieldError(field string, message string) error {
	return &FieldError{
		Field: field,
		Err:   errors.New(message),
	}
}

// validateNested recursively validates structs that implement Validator interface
func validateNested(obj interface{}, path string) error {
	val := reflect.ValueOf(obj)

	// Skip nil pointers entirely
	if val.Kind() == reflect.Ptr && val.IsNil() {
		return nil
	}

	// Check if object itself implements Validator (but not if it's a nil pointer)
	if v, ok := obj.(Validator); ok {
		if err := v.Validate(); err != nil {
			return &FieldError{Field: path, Err: err}
		}
	}

	// Dereference pointer for struct field iteration
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// Recursively validate struct fields
	if val.Kind() == reflect.Struct {
		for i := 0; i < val.NumField(); i++ {
			field := val.Field(i)
			fieldType := val.Type().Field(i)
			if !fieldType.IsExported() || !field.CanInterface() {
				continue
			}

			// Build field path using JSON tag name
			fieldName := fieldType.Name
			if jsonTag := fieldType.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
				// Extract field name from json tag (handle "name,omitempty" case)
				if idx := len(jsonTag); idx > 0 {
					for j, c := range jsonTag {
						if c == ',' {
							idx = j
							break
						}
					}
					fieldName = jsonTag[:idx]
				}
			}

			fieldPath := fieldName
			if path != "" {
				fieldPath = path + "." + fieldName
			}

			// Check if field value or pointer to field implements Validator
			fieldInterface := field.Interface()
			if field.CanAddr() {
				// Try pointer to field first (for pointer receiver methods)
				if v, ok := field.Addr().Interface().(Validator); ok {
					if err := v.Validate(); err != nil {
						return &FieldError{Field: fieldPath, Err: err}
					}
				}
			}

			if err := validateNested(fieldInterface, fieldPath); err != nil {
				return err
			}
		}
		return nil
	}

	// Recursively validate slice/array elements
	if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
		for i := 0; i < val.Len(); i++ {
			elem := val.Index(i)
			elemPath := fmt.Sprintf("%s[%d]", path, i)

			// Check if element or pointer to element implements Validator
			if elem.CanAddr() {
				// Try pointer to element first (for pointer receiver methods)
				if v, ok := elem.Addr().Interface().(Validator); ok {
					if err := v.Validate(); err != nil {
						return &FieldError{Field: elemPath, Err: err}
					}
				}
			}

			if err := validateNested(elem.Interface(), elemPath); err != nil {
				return err
			}
		}
		return nil
	}

	// Recursively validate map values
	if val.Kind() == reflect.Map {
		iter := val.MapRange()
		for iter.Next() {
			mapVal := iter.Value()
			keyPath := fmt.Sprintf("%s[%v]", path, iter.Key().Interface())

			// For map values, we can't take address, so only check value directly
			if v, ok := mapVal.Interface().(Validator); ok {
				if err := v.Validate(); err != nil {
					return &FieldError{Field: keyPath, Err: err}
				}
			}

			if err := validateNested(mapVal.Interface(), keyPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// handleValidationError converts errors to ValidationError slice
func handleValidationError(err error) []ValidationError {
	validationErrors := []ValidationError{}

	// Check for field-specific error from nested validation
	var fieldErr *FieldError
	if errors.As(err, &fieldErr) {
		// Check if the underlying error is also a FieldError (user-created with NewFieldError)
		var innerFieldErr *FieldError
		if errors.As(fieldErr.Err, &innerFieldErr) {
			// Combine paths: outer.inner
			combinedPath := innerFieldErr.Field
			if fieldErr.Field != "" && innerFieldErr.Field != "" {
				combinedPath = fieldErr.Field + "." + innerFieldErr.Field
			} else if fieldErr.Field != "" {
				combinedPath = fieldErr.Field
			}

			// Use "custom" for empty combined path
			if combinedPath == "" {
				combinedPath = "custom"
			}

			validationErrors = append(validationErrors, ValidationError{
				Field:   combinedPath,
				Message: innerFieldErr.Err.Error(),
			})
			return validationErrors
		}

		// Check if the underlying error is validator.ValidationErrors
		var verr validator.ValidationErrors
		if errors.As(fieldErr.Err, &verr) {
			for _, f := range verr {
				fieldPath := f.Field()
				if fieldErr.Field != "" {
					fieldPath = fieldErr.Field + "." + f.Field()
				}
				validationErrors = append(validationErrors, ValidationError{
					Field:   fieldPath,
					Message: fmt.Sprintf(fieldErrMsg, f.Tag()),
				})
			}
		} else {
			// Use "custom" for empty field path (top-level validation)
			fieldName := fieldErr.Field
			if fieldName == "" {
				fieldName = "custom"
			}
			validationErrors = append(validationErrors, ValidationError{
				Field:   fieldName,
				Message: fieldErr.Err.Error(),
			})
		}
		return validationErrors
	}

	// Check for validator.ValidationErrors at top level
	var verr validator.ValidationErrors
	if errors.As(err, &verr) {
		for _, f := range verr {
			validationErrors = append(validationErrors, ValidationError{
				Field:   f.Field(),
				Message: fmt.Sprintf(fieldErrMsg, f.Tag()),
			})
		}
	} else {
		validationErrors = append(validationErrors, ValidationError{
			Field:   "custom",
			Message: err.Error(),
		})
	}

	return validationErrors
}

// ValidateJSON validates an incoming JSON request against a struct with validation tags
// It performs two-stage validation:
// 1. Binding validation using `binding` tags
// 2. Recursive custom validation for structs implementing Validator interface
//
// Error responses include full field paths for nested structures (e.g., "address.zip_code")
//
// Example usage:
//
//	type Address struct {
//	    Street  string `json:"street" binding:"required"`
//	    ZipCode string `json:"zip_code" binding:"required,len=5"`
//	}
//
//	func (a *Address) Validate() error {
//	    if a.ZipCode == "00000" {
//	        return NewFieldError("zip_code", "invalid zip code")
//	    }
//	    return nil
//	}
//
//	type LoginRequest struct {
//	    Username string  `json:"username" binding:"required,email"`
//	    Password string  `json:"password" binding:"required,min=8,max=32,securepassword"`
//	    Address  Address `json:"address" binding:"required"`
//	}
//
//	func (r *LoginRequest) Validate() error {
//	    if r.Username == "admin" && len(r.Password) < 12 {
//	        // Using NewFieldError for field-specific errors
//	        return NewFieldError("password", "admin password must be at least 12 characters")
//	    }
//	    // Or return generic error for top-level validation
//	    return nil
//	}
//
//	// Error response examples:
//	// Field-specific: {"errors": [{"field": "password", "message": "admin password must be at least 12 characters"}]}
//	// Nested field: {"errors": [{"field": "address.zip_code", "message": "invalid zip code"}]}
//	// Generic: {"errors": [{"field": "custom", "message": "validation failed"}]}
//
//	func LoginHandler(c *gin.Context) {
//	    var req LoginRequest
//	    if !ValidateJSON(c, &req) {
//	        return // Validation failed and error response already sent
//	    }
//	    // Continue with valid request
//	}
func ValidateJSON(c *gin.Context, obj interface{}) bool {
	// Stage 1: Binding validation using `binding` tags
	if err := c.ShouldBindJSON(obj); err != nil {
		var validationErrors []ValidationError

		// Check if it's validator.ValidationErrors
		var verr validator.ValidationErrors
		if errors.As(err, &verr) {
			validationErrors = handleValidationError(err)
		} else {
			// Handle other binding errors (JSON unmarshal, type conversion, etc.)
			validationErrors = []ValidationError{{
				Field:   "body",
				Message: err.Error(),
			}}
		}

		response.ValidationError(c, validationErrors)
		return false
	}

	// Stage 2: Recursive custom validation
	if err := validateNested(obj, ""); err != nil {
		response.ValidationError(c, handleValidationError(err))
		return false
	}

	return true
}

// ValidateQuery validates URL query parameters against a struct with validation tags
// It performs two-stage validation:
// 1. Binding validation using `binding` tags
// 2. Recursive custom validation for structs implementing Validator interface
//
// # Error responses include full field paths for nested structures
//
// Example usage:
//
//	type SearchRequest struct {
//	    Query    string `form:"q" binding:"required,min=3"`
//	    Page     int    `form:"page" binding:"min=1"`
//	    PageSize int    `form:"page_size" binding:"min=10,max=100"`
//	}
//
//	func (s *SearchRequest) Validate() error {
//	    if s.Page > 1000 {
//	        return NewFieldError("page", "maximum page number is 1000")
//	    }
//	    return nil
//	}
//
//	func SearchHandler(c *gin.Context) {
//	    var req SearchRequest
//	    if !ValidateQuery(c, &req) {
//	        return // Validation failed and error response already sent
//	    }
//	    // Continue with valid request
//	}
func ValidateQuery(c *gin.Context, obj interface{}) bool {
	// Stage 1: Binding validation using `binding` tags
	if err := c.ShouldBindQuery(obj); err != nil {
		var validationErrors []ValidationError

		// Check if it's validator.ValidationErrors
		var verr validator.ValidationErrors
		if errors.As(err, &verr) {
			validationErrors = handleValidationError(err)
		} else {
			// Handle other binding errors (type conversion, etc.)
			// Use "-" for backward compatibility with previous implementation
			validationErrors = []ValidationError{{
				Field:   "-",
				Message: err.Error(),
			}}
		}

		response.ValidationError(c, validationErrors)
		return false
	}

	// Stage 2: Recursive custom validation
	if err := validateNested(obj, ""); err != nil {
		response.ValidationError(c, handleValidationError(err))
		return false
	}

	return true
}

func init() {
	// Register custom validators with Gin's validator instance
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		// Register tag name function to use JSON field names in error messages
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := fld.Tag.Get("json")
			if name == "" || name == "-" {
				return fld.Name
			}
			// Handle "fieldname,omitempty" case
			if idx := strings.Index(name, ","); idx != -1 {
				name = name[:idx]
			}
			return name
		})

		if err := v.RegisterValidation("securepassword", cv.ValidateSecurePassword); err != nil {
			panic(err)
		}
	}
}
