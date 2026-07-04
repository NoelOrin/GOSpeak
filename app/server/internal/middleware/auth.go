package middleware

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/redis"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// permissionChecker 抽象权限查询，便于测试和解耦。
type permissionChecker interface {
	HasPermission(roleName, permCode string) bool
}

// tokenVersionChecker 抽象 token 版本查询，供 JWTAuth 校验改密后旧 token 是否失效。
type tokenVersionChecker interface {
	GetTokenVersionByUUID(userUUID string) (uint, error)
}

var (
	permChecker       permissionChecker
	tokenVersionCheck tokenVersionChecker
)

// SetPermissionChecker 注入权限查询实现（启动时调用）。
func SetPermissionChecker(c permissionChecker) {
	permChecker = c
}

// SetTokenVersionChecker 注入 token 版本查询实现（启动时调用）。
func SetTokenVersionChecker(tvc tokenVersionChecker) {
	tokenVersionCheck = tvc
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

		// 校验 token 版本：改密/重置后用户 TokenVersion 递增，旧 token 返回 TOKEN_REVOKED。
		if tokenVersionCheck != nil && claims.UserUUID != "" {
			currentVersion, err := tokenVersionCheck.GetTokenVersionByUUID(claims.UserUUID)
			if err != nil || currentVersion != claims.TokenVersion {
				pkg.Fail(c, pkg.TOKEN_REVOKED)
				c.Abort()
				return
			}
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
		owner, ownerOK := c.Get(ownerContextKey)
		username, usernameOK := c.Get("username")
		ownerStr, ownerIsString := owner.(string)
		usernameStr, usernameIsString := username.(string)
		if ownerOK && usernameOK && ownerIsString && usernameIsString && ownerStr == usernameStr {
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
		allowed := os.Getenv("CORS_ORIGIN")
		if allowed == "" {
			allowed = "*"
		}
		c.Header("Access-Control-Allow-Origin", allowed)
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
