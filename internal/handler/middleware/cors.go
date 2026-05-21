package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func CORS(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowOrigin := "*"

		// 若已配置白名单但未带 Origin（curl/服务端客户端等），不按浏览器 CORS 规则拦截
		if len(allowedOrigins) > 0 && strings.TrimSpace(origin) == "" {
			c.Next()
			return
		}

		// 如果配置了允许的来源，则检查 origin 是否在列表中
		if len(allowedOrigins) > 0 {
			allowed := false
			for _, ao := range allowedOrigins {
				if origin == ao {
					allowed = true
					break
				}
			}
			if allowed {
				allowOrigin = origin
			} else {
				// 不在允许列表中，返回 403
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}

		c.Header("Access-Control-Allow-Origin", allowOrigin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		c.Header("Access-Control-Max-Age", "86400")
		if allowOrigin != "*" {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
