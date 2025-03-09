package httpserver

import (
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"net/http"
	"strings"
)

var (
	// Global validator instance
	validate = validator.New()
)

// ValidationError represents a field validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationResponse is the standard format for validation errors
type ValidationResponse struct {
	Success bool              `json:"success"`
	Errors  []ValidationError `json:"errors"`
}

// ValidateJSON validates an incoming JSON request against a struct with validation tags
// Example usage:
//
//  type LoginRequest struct {
//      Username string `json:"username" binding:"required" validate:"email"`
//      Password string `json:"password" binding:"required" validate:"min=8,max=32"`
//  }
//
//  func LoginHandler(c *gin.Context) {
//      var req LoginRequest
//      if !ValidateJSON(c, &req) {
//          return // Validation failed and error response already sent
//      }
//      // Continue with valid request
//  }
func ValidateJSON(c *gin.Context, obj interface{}) bool {
	// First use Gin's binding to check basic requirements
	if err := c.ShouldBindJSON(obj); err != nil {
		validationErrors := []ValidationError{}
		
		// Extract field error details from Gin's binding errors
		if verr, ok := err.(validator.ValidationErrors); ok {
			for _, f := range verr {
				// Format the field name and error message
				field := strings.ToLower(f.Field())
				message := formatValidationMessage(f)
				validationErrors = append(validationErrors, ValidationError{
					Field:   field,
					Message: message,
				})
			}
		} else {
			// If not a validation error, it's likely a malformed JSON
			validationErrors = append(validationErrors, ValidationError{
				Field:   "request",
				Message: "Invalid JSON format",
			})
		}
		
		// Return validation errors in standard format
		c.AbortWithStatusJSON(http.StatusBadRequest, ValidationResponse{
			Success: false,
			Errors:  validationErrors,
		})
		return false
	}
	
	// Run additional validations with the full validator
	if err := validate.Struct(obj); err != nil {
		validationErrors := []ValidationError{}
		
		if verr, ok := err.(validator.ValidationErrors); ok {
			for _, f := range verr {
				field := strings.ToLower(f.Field())
				message := formatValidationMessage(f)
				validationErrors = append(validationErrors, ValidationError{
					Field:   field,
					Message: message,
				})
			}
		}
		
		c.AbortWithStatusJSON(http.StatusBadRequest, ValidationResponse{
			Success: false,
			Errors:  validationErrors,
		})
		return false
	}
	
	return true
}

// ValidateQuery validates URL query parameters against a struct with validation tags
func ValidateQuery(c *gin.Context, obj interface{}) bool {
	if err := c.ShouldBindQuery(obj); err != nil {
		validationErrors := []ValidationError{}
		
		if verr, ok := err.(validator.ValidationErrors); ok {
			for _, f := range verr {
				field := strings.ToLower(f.Field())
				message := formatValidationMessage(f)
				validationErrors = append(validationErrors, ValidationError{
					Field:   field,
					Message: message,
				})
			}
		} else {
			validationErrors = append(validationErrors, ValidationError{
				Field:   "query",
				Message: "Invalid query parameters",
			})
		}
		
		c.AbortWithStatusJSON(http.StatusBadRequest, ValidationResponse{
			Success: false,
			Errors:  validationErrors,
		})
		return false
	}
	
	return true
}

// Helper function to format validation error messages
func formatValidationMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "This field is required"
	case "email":
		return "Invalid email format"
	case "min":
		return "Value is too short"
	case "max":
		return "Value is too long"
	case "alphanum":
		return "Only alphanumeric characters are allowed"
	default:
		return "Invalid value"
	}
}

// CustomValidationRules registers custom validation rules
func CustomValidationRules() {
	// Register custom validation rules
	validate.RegisterValidation("securepassword", validateSecurePassword)
	// Add more custom validations as needed
}

// Sample custom validation rule for secure passwords
func validateSecurePassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()
	
	// Password complexity requirements
	hasUpperCase := false
	hasLowerCase := false
	hasNumber := false
	hasSpecial := false
	
	if len(password) < 8 {
		return false
	}
	
	for _, char := range password {
		switch {
		case 'A' <= char && char <= 'Z':
			hasUpperCase = true
		case 'a' <= char && char <= 'z':
			hasLowerCase = true
		case '0' <= char && char <= '9':
			hasNumber = true
		case strings.ContainsRune(`!@#$%^&*()-_=+[]{}|;:'",.<>/?`, char):
			hasSpecial = true
		}
	}
	
	// Require at least 3 of the 4 character types
	count := 0
	if hasUpperCase {
		count++
	}
	if hasLowerCase {
		count++
	}
	if hasNumber {
		count++
	}
	if hasSpecial {
		count++
	}
	
	return count >= 3
}

// Initialize validators
func init() {
	CustomValidationRules()
}