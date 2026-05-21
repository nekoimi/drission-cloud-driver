package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func RequestLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Duration("latency", latency),
			zap.String("ip", c.ClientIP()),
		}

		if requestID, exists := c.Get(RequestIDKey); exists {
			fields = append(fields, zap.String("request_id", requestID.(string)))
		}

		if userID, exists := c.Get("userID"); exists {
			fields = append(fields, zap.String("user_id", userID.(string)))
		}

		if query != "" {
			fields = append(fields, zap.String("query", query))
		}

		if status >= 500 {
			logger.Error("request", fields...)
		} else if status >= 400 {
			logger.Warn("request", fields...)
		} else {
			logger.Info("request", fields...)
		}
	}
}
