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

// VerifyToken 执行完整的 token 校验链：签名解析 → 过期 → 黑名单 → 版本。
// 供 JWTAuth / WSAuth / signal.OnConnect 共用，保证校验口径一致。
func VerifyToken(tokenStr string) (*pkg.Claims, pkg.ErrCode) {
	claims, err := pkg.ParseToken(tokenStr)
	if err != nil {
		return nil, pkg.TOKEN_WRONG
	}
	if pkg.IsTokenExpired(claims) {
		return nil, pkg.TOKEN_EXPIRED
	}
	if redis.IsBlacklisted(claims.ID) {
		return nil, pkg.TOKEN_REVOKED
	}
	if tokenVersionCheck != nil && claims.UserUUID != "" {
		currentVersion, err := tokenVersionCheck.GetTokenVersionByUUID(claims.UserUUID)
		if err != nil {
			return nil, pkg.INTERNAL_ERROR
		}
		if currentVersion != claims.TokenVersion {
			return nil, pkg.TOKEN_REVOKED
		}
	}
	return claims, pkg.SUCCESS
}

func setClaimsContext(c *gin.Context, claims *pkg.Claims) {
	c.Set("username", claims.Username)
	c.Set("display_name", claims.DisplayName)
	c.Set("user_uuid", claims.UserUUID)
	c.Set("role", claims.Role)
	c.Set("claims", claims)
	c.Set("auth_type", "jwt")
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

		claims, code := VerifyToken(tokenStr)
		if code != pkg.SUCCESS {
			pkg.Fail(c, code)
			c.Abort()
			return
		}

		setClaimsContext(c, claims)
		c.Next()
	}
}

// WSAuth 为 Socket.IO 路由提供 WebSocket 鉴权门控。
// 与 JWTAuth 共用 VerifyToken，从 URL query 读取 token（浏览器 WS 握手不支持自定义头）。
func WSAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.Query("token")
		if tokenStr == "" {
			pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
			c.Abort()
			return
		}

		claims, code := VerifyToken(tokenStr)
		if code != pkg.SUCCESS {
			pkg.Fail(c, code)
			c.Abort()
			return
		}

		setClaimsContext(c, claims)
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

// permissionGranted 判断当前 caller 是否拥有 permCode。
func permissionGranted(c *gin.Context, permCode string) bool {
	var claims *pkg.Claims
	if v, ok := c.Get("claims"); ok {
		claims, _ = v.(*pkg.Claims)
	}
	role, _ := c.Get("role")
	roleStr, _ := role.(string)
	return PermissionGranted(claims, roleStr, permCode, permChecker)
}

// PermissionGranted 统一权限判定：
// 1) claims.Permissions 非空时，仅信任 token 显式权限（Bot / 细粒度 token）；
// 2) 否则回退 role → checker 映射（普通用户）。
// 供 HTTP 中间件与 Socket.IO Hub 共用，避免踢人/禁言路径口径不一致。
func PermissionGranted(claims *pkg.Claims, role string, permCode string, checker permissionChecker) bool {
	if claims != nil && len(claims.Permissions) > 0 {
		for _, p := range claims.Permissions {
			if p == permCode {
				return true
			}
		}
		return false
	}
	return checker != nil && checker.HasPermission(role, permCode)
}

// RequirePermission 基于权限码的鉴权，替代硬编码角色名。
func RequirePermission(permCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, exists := c.Get("role")
		if !exists {
			pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
			c.Abort()
			return
		}
		if permissionGranted(c, permCode) {
			c.Next()
			return
		}

		pkg.Fail(c, pkg.FORBIDDEN)
		c.Abort()
	}
}

// RequireOwnerOrPermission 资源归属校验：拥有资源归属 或 拥有指定权限码 即可放行。
// ownerField 从 gin.Context 中取到的 owner 标识（如 username），
// 与 JWT 中的 username 比对。
func RequireOwnerOrPermission(ownerContextKey, permCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if permissionGranted(c, permCode) {
			c.Next()
			return
		}

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
