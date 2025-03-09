package httpserver

import (
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"net/http"
)

// IsJSONRequest returns true if request is a JSON request
func IsJSONRequest(ctx *gin.Context) bool {
	return ctx.Request.Header.Get(HeaderAccept) == ContentTypeJson ||
		ctx.Request.Header.Get(HeaderContentType) == ContentTypeJson
}

// HttpError401 generates a error 401 response with logging
func HttpError401(ctx *gin.Context) {
	// Log the unauthorized access attempt
	log.RequestWarn(ctx, "Unauthorized access attempt", map[string]interface{}{
		"status": http.StatusUnauthorized,
	})
	
	if IsJSONRequest(ctx) {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, JSONResponseError{
			Success: false,
			Error: JSONErrorDetail{
				Message: http.StatusText(http.StatusUnauthorized),
			},
		})
		return
	}
	ctx.AbortWithStatus(http.StatusUnauthorized)
}

// HttpError403 generates a 403 Forbidden response with logging
func HttpError403(ctx *gin.Context) {
	// Log the forbidden access attempt
	log.RequestWarn(ctx, "Forbidden access attempt", map[string]interface{}{
		"status": http.StatusForbidden,
	})
	
	if IsJSONRequest(ctx) {
		ctx.AbortWithStatusJSON(http.StatusForbidden, JSONResponseError{
			Success: false,
			Error: JSONErrorDetail{
				Message: http.StatusText(http.StatusForbidden),
			},
		})
		return
	}
	ctx.AbortWithStatus(http.StatusForbidden)
}

// HttpError404 generates a 404 Not Found response with logging
func HttpError404(ctx *gin.Context) {
	// Log the not found request
	log.RequestInfo(ctx, "Resource not found", map[string]interface{}{
		"status": http.StatusNotFound,
	})
	
	if IsJSONRequest(ctx) {
		ctx.AbortWithStatusJSON(http.StatusNotFound, JSONResponseError{
			Success: false,
			Error: JSONErrorDetail{
				Message: http.StatusText(http.StatusNotFound),
			},
		})
		return
	}
	ctx.AbortWithStatus(http.StatusNotFound)
}

// HttpError400 generates a 400 Bad Request response with logging
func HttpError400(ctx *gin.Context, message string) {
	if message == "" {
		message = http.StatusText(http.StatusBadRequest)
	}
	
	// Log the bad request
	log.RequestWarn(ctx, "Bad request", map[string]interface{}{
		"status":  http.StatusBadRequest,
		"message": message,
	})
	
	if IsJSONRequest(ctx) {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, JSONResponseError{
			Success: false,
			Error: JSONErrorDetail{
				Message: message,
			},
		})
		return
	}
	ctx.AbortWithStatus(http.StatusBadRequest)
}

// HttpError500 generates a 500 Internal Server Error response with logging
func HttpError500(ctx *gin.Context, err error) {
	// Log the server error with stack trace
	log.RequestError(ctx, err, "Internal server error", map[string]interface{}{
		"status": http.StatusInternalServerError,
	})
	
	// Don't expose internal error details to the client
	if IsJSONRequest(ctx) {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, JSONResponseError{
			Success: false,
			Error: JSONErrorDetail{
				Message: http.StatusText(http.StatusInternalServerError),
			},
		})
		return
	}
	ctx.AbortWithStatus(http.StatusInternalServerError)
}

// HttpSuccess sends a successful response with logging
func HttpSuccess(ctx *gin.Context, data interface{}) {
	// Log the successful request at debug level to avoid log pollution
	log.RequestDebug(ctx, "Request completed successfully", map[string]interface{}{
		"status": http.StatusOK,
	})
	
	if IsJSONRequest(ctx) {
		ctx.JSON(http.StatusOK, JSONResponseSuccess{
			Success: true,
			Data:    data,
		})
		return
	}
	
	ctx.Status(http.StatusOK)
}
