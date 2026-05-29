package middleware

import (
	"go_rtc/internal/model"
	"go_rtc/internal/pkg"
	"go_rtc/internal/redis"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// permissionChecker 抽象权限查询，便于测试和解耦。
type permissionChecker interface {
	HasPermission(roleName, permCode string) bool
}

var permChecker permissionChecker

// SetPermissionChecker 注入权限查询实现（启动时调用）。
func SetPermissionChecker(c permissionChecker) {
	permChecker = c
}

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

		if redis.IsBlacklisted(claims.ID) {
			pkg.Fail(c, pkg.TOKEN_REVOKED)
			c.Abort()
			return
		}

		c.Set("username", claims.Username)
		c.Set("display_name", claims.DisplayName)
		c.Set("user_uuid", claims.UserUUID)
		c.Set("role", claims.Role)
		c.Set("claims", claims)
		c.Next()
	}
}

func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
			c.Abort()
			return
		}

		roleStr, ok := role.(string)
		if !ok {
			pkg.Fail(c, pkg.INTERNAL_ERROR)
			c.Abort()
			return
		}

		for _, allowedRole := range roles {
			if roleStr == allowedRole {
				c.Next()
				return
			}
		}

		pkg.Fail(c, pkg.FORBIDDEN)
		c.Abort()
	}
}

// RequirePermission 基于权限码的鉴权，替代硬编码角色名。
func RequirePermission(permCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
			c.Abort()
			return
		}
		roleStr, ok := role.(string)
		if !ok {
			pkg.Fail(c, pkg.FORBIDDEN)
			c.Abort()
			return
		}

		if permChecker == nil || !permChecker.HasPermission(roleStr, permCode) {
			pkg.Fail(c, pkg.FORBIDDEN)
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireOwnerOrPermission 资源归属校验：拥有资源归属 或 拥有指定权限码 即可放行。
// ownerField 从 gin.Context 中取到的 owner 标识（如 username），
// 与 JWT 中的 username 比对。
func RequireOwnerOrPermission(ownerContextKey, permCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 先尝试权限码放行
		role, _ := c.Get("role")
		roleStr, _ := role.(string)
		if permChecker != nil && permChecker.HasPermission(roleStr, permCode) {
			c.Next()
			return
		}

		// 再尝试归属放行
		owner, _ := c.Get(ownerContextKey)
		username, _ := c.Get("username")
		if owner != nil && username != nil && owner.(string) == username.(string) {
			c.Next()
			return
		}

		pkg.Fail(c, pkg.FORBIDDEN)
		c.Abort()
	}
}

func BanCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.Next()
			return
		}
		roleStr, ok := role.(string)
		if !ok {
			c.Next()
			return
		}
		if model.HasBanRole(roleStr) {
			pkg.Fail(c, pkg.USER_BANNED)
			c.Abort()
			return
		}
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