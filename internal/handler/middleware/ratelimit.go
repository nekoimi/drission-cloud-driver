package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/nekoimi/go-project-template/internal/pkg/errcode"
	"github.com/nekoimi/go-project-template/internal/pkg/response"
)

// RateLimit 全局令牌桶限流中间件
func RateLimit(rps float64, burst int) gin.HandlerFunc {
	limiter := rate.NewLimiter(rate.Limit(rps), burst)

	return func(c *gin.Context) {
		if !limiter.Allow() {
			response.Error(c, http.StatusTooManyRequests, errcode.TooManyReq)
			c.Abort()
			return
		}
		c.Next()
	}
}

type limiterEntry struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// IPRateLimit 基于 IP 的令牌桶限流（带过期清理）
func IPRateLimit(rps float64, burst int) gin.HandlerFunc {
	var mu sync.Mutex
	limiters := make(map[string]*limiterEntry)

	// 定期清理过期的 limiter
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			cutoff := time.Now().Add(-10 * time.Minute)
			for ip, entry := range limiters {
				if entry.lastAccess.Before(cutoff) {
					delete(limiters, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()

		mu.Lock()
		entry, exists := limiters[ip]
		if !exists {
			entry = &limiterEntry{
				limiter:    rate.NewLimiter(rate.Limit(rps), burst),
				lastAccess: time.Now(),
			}
			limiters[ip] = entry
		} else {
			entry.lastAccess = time.Now()
		}
		mu.Unlock()

		if !entry.limiter.Allow() {
			response.Error(c, http.StatusTooManyRequests, errcode.TooManyReq)
			c.Abort()
			return
		}
		c.Next()
	}
}
