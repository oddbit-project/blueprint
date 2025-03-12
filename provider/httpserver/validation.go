package httpserver

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	cv "github.com/oddbit-project/blueprint/provider/httpserver/request/validator"
	"github.com/oddbit-project/blueprint/provider/httpserver/response"
)

const (
	fieldErrMsg = "Error: Field validation failed on the '%s' validator"
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

// ValidateJSON validates an incoming JSON request against a struct with validation tags
// Example usage:
//
//	type LoginRequest struct {
//	    Username string `json:"username" binding:"required" validate:"email"`
//	    Password string `json:"password" binding:"required" validate:"min=8,max=32"`
//	}
//
//	func LoginHandler(c *gin.Context) {
//	    var req LoginRequest
//	    if !ValidateJSON(c, &req) {
//	        return // Validation failed and error response already sent
//	    }
//	    // Continue with valid request
//	}
func ValidateJSON(c *gin.Context, obj interface{}) bool {
	// First use Gin's binding to check basic requirements
	if err := c.ShouldBindJSON(obj); err != nil {
		validationErrors := []ValidationError{}

		// Extract field error details from Gin's binding errors
		var verr validator.ValidationErrors
		if errors.As(err, &verr) {
			for _, f := range verr {
				validationErrors = append(validationErrors, ValidationError{
					Field:   f.Field(),
					Message: fmt.Sprintf(fieldErrMsg, f.Tag()),
				})
			}
		}

		response.ValidationError(c, validationErrors)
		return false
	}

	// Run additional validations with the full validator
	if err := validate.Struct(obj); err != nil {
		validationErrors := []ValidationError{}

		var verr validator.ValidationErrors
		if errors.As(err, &verr) {
			for _, f := range verr {
				validationErrors = append(validationErrors, ValidationError{
					Field:   f.Field(),
					Message: fmt.Sprintf(fieldErrMsg, f.Tag()),
				})
			}
		}

		response.ValidationError(c, validationErrors)
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
				validationErrors = append(validationErrors, ValidationError{
					Field:   f.Field(),
					Message: fmt.Sprintf(fieldErrMsg, f.Tag()),
				})
			}
		} else {
			validationErrors = append(validationErrors, ValidationError{
				Field:   "-",
				Message: "Invalid query parameters",
			})
		}

		response.ValidationError(c, validationErrors)
		return false
	}

	return true
}

func init() {
	// register custom validators
	if err := validate.RegisterValidation("securepassword", cv.ValidateSecurePassword); err != nil {
		panic(err)
	}

}
