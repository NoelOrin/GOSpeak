package middleware

import (
	"go_rtc/internal/pkg"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenHeader := c.Request.Header.Get("Authorization")
		if tokenHeader == "" {
			pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
			c.Abort()
			return
		}

		tokenStr := strings.TrimPrefix(tokenHeader, "Bearer ")
		if tokenStr == tokenHeader {
			tokenStr = tokenHeader
		}

		claims, err := pkg.ParseToken(tokenStr)
		if err != nil {
			pkg.Fail(c, pkg.TOKEN_WRONG)
			c.Abort()
			return
		}

		if pkg.IsTokenExpired(claims) {
			pkg.Fail(c, pkg.TOKEN_EXPIRED)
			c.Abort()
			return
		}

		c.Set("username", claims.Username)
		c.Set("user_uuid", claims.UserUUID)
		c.Set("claims", claims)
		c.Next()
	}
}

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}