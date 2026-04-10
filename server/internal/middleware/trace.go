package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TraceID adds a request ID to each request and response header
func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try to get from request header first (in case of upstream gateway)
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			// Generate new UUID v4
			requestID = uuid.New().String()
		}

		// Set in Gin context
		c.Set("requestID", requestID)

		// Set in response header
		c.Header("X-Request-ID", requestID)

		c.Next()
	}
}
