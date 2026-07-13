package middleware

import (
	"crypto/rand"
	"fmt"

	"github.com/gin-gonic/gin"
)

const RequestIDKey = "X-Request-ID"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(RequestIDKey)
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Set(RequestIDKey, requestID)
		c.Header(RequestIDKey, requestID)
		c.Next()
	}
}

func generateRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
