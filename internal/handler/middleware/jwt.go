package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/nekoimi/go-project-template/internal/pkg/errcode"
	"github.com/nekoimi/go-project-template/internal/pkg/jwtutil"
	"github.com/nekoimi/go-project-template/internal/pkg/response"
)

func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Error(c, http.StatusUnauthorized, errcode.Unauthorized)
			c.Abort()
			return
		}

		parts := splitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Error(c, http.StatusUnauthorized, errcode.Unauthorized)
			c.Abort()
			return
		}

		userID, err := jwtutil.ValidateToken(parts[1], secret)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, errcode.Unauthorized)
			c.Abort()
			return
		}

		c.Set("userID", userID)
		c.Next()
	}
}

// splitN 按分隔符分割字符串，最多返回 n 部分
func splitN(s, sep string, n int) []string {
	if n <= 0 {
		return nil
	}
	if sep == "" {
		return []string{s}
	}

	var parts []string
	for i := 0; i < n-1; i++ {
		idx := index(s, sep)
		if idx == -1 {
			break
		}
		parts = append(parts, s[:idx])
		s = s[idx+len(sep):]
	}
	parts = append(parts, s)
	return parts
}

func index(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
