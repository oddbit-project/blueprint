package response

import (
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/provider/httpserver/log"
	"github.com/oddbit-project/blueprint/provider/httpserver/request"
	"net/http"
)

// Http401 generates a error 401 response with logging
func Http401(ctx *gin.Context) {
	// Log the unauthorized access attempt
	log.RequestWarn(ctx, "Unauthorized access attempt", map[string]interface{}{
		"status": http.StatusUnauthorized,
	})

	if request.IsJSONRequest(ctx) {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, JSONResponseError{
			Success: false,
			Error: ErrorDetail{
				Message: http.StatusText(http.StatusUnauthorized),
			},
		})
		return
	}
	ctx.AbortWithStatus(http.StatusUnauthorized)
}

// Http403 generates a 403 Forbidden response with logging
func Http403(ctx *gin.Context) {
	// Log the forbidden access attempt
	log.RequestWarn(ctx, "Forbidden access attempt", map[string]interface{}{
		"status": http.StatusForbidden,
	})

	if request.IsJSONRequest(ctx) {
		ctx.AbortWithStatusJSON(http.StatusForbidden, JSONResponseError{
			Success: false,
			Error: ErrorDetail{
				Message: http.StatusText(http.StatusForbidden),
			},
		})
		return
	}
	ctx.AbortWithStatus(http.StatusForbidden)
}

// Http404 generates a 404 Not Found response with logging
func Http404(ctx *gin.Context) {
	// Log the not found request
	log.RequestInfo(ctx, "Resource not found", map[string]interface{}{
		"status": http.StatusNotFound,
	})

	if request.IsJSONRequest(ctx) {
		ctx.AbortWithStatusJSON(http.StatusNotFound, JSONResponseError{
			Success: false,
			Error: ErrorDetail{
				Message: http.StatusText(http.StatusNotFound),
			},
		})
		return
	}
	ctx.AbortWithStatus(http.StatusNotFound)
}

// Http400 generates a 400 Bad Request response with logging
func Http400(ctx *gin.Context, message string) {
	if message == "" {
		message = http.StatusText(http.StatusBadRequest)
	}

	// Log the bad request
	log.RequestWarn(ctx, "Bad request", map[string]interface{}{
		"status":  http.StatusBadRequest,
		"message": message,
	})

	if request.IsJSONRequest(ctx) {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, JSONResponseError{
			Success: false,
			Error: ErrorDetail{
				Message: message,
			},
		})
		return
	}
	ctx.AbortWithStatus(http.StatusBadRequest)
}

// Http500 generates a 500 Internal Server Error response with logging
func Http500(ctx *gin.Context, err error) {
	// Log the server error with stack trace
	log.RequestError(ctx, err, "Internal server error", map[string]interface{}{
		"status": http.StatusInternalServerError,
	})

	// Don't expose internal error details to the client
	if request.IsJSONRequest(ctx) {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, JSONResponseError{
			Success: false,
			Error: ErrorDetail{
				Message: http.StatusText(http.StatusInternalServerError),
			},
		})
		return
	}
	ctx.AbortWithStatus(http.StatusInternalServerError)
}

// ValidationError generates a 400 Bad Request response with validation failed details
func ValidationError(ctx *gin.Context, errors interface{}) {
	message := "request validation failed"
	// Log the bad request
	log.RequestWarn(ctx, "validation failed", map[string]interface{}{
		"status":  http.StatusBadRequest,
		"message": message,
	})

	if request.IsJSONRequest(ctx) {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, JSONResponseError{
			Success: false,
			Error: ErrorDetail{
				Message:      message,
				RequestError: errors,
			},
		})
		return
	}
	ctx.AbortWithStatus(http.StatusBadRequest)
}
