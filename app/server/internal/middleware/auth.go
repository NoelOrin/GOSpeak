package middleware

import (
	"GOSpeak/internal/config"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/redis"
	"net/http"
	"strings"
	"sync"
	"time"

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

// botTokenChecker 校验 Bot token 的 DB 吊销/过期状态，补充 TokenVersion 之外的 Revoked 双源。
type botTokenChecker interface {
	IsBotTokenValid(userUUID string) (bool, error)
}

var (
	permChecker       permissionChecker
	tokenVersionCheck tokenVersionChecker
	botTokenCheck     botTokenChecker
)

// tokenVersionCacheTTL 缩短 DB 抖动对每个请求的影响；改密/换角色后最多延迟一个 TTL 生效。
const tokenVersionCacheTTL = 500 * time.Millisecond

type tokenVersionCacheEntry struct {
	version uint
	expires time.Time
}

var tokenVersionCache = struct {
	sync.Mutex
	m map[string]tokenVersionCacheEntry
}{m: make(map[string]tokenVersionCacheEntry)}

// SetPermissionChecker 注入权限查询实现（启动时调用）。
func SetPermissionChecker(c permissionChecker) {
	permChecker = c
}

// SetTokenVersionChecker 注入 token 版本查询实现（启动时调用）。
func SetTokenVersionChecker(tvc tokenVersionChecker) {
	tokenVersionCheck = tvc
}

// SetBotTokenChecker 注入 Bot token 吊销/过期校验实现（启动时调用）。
func SetBotTokenChecker(btc botTokenChecker) {
	botTokenCheck = btc
}

// VerifyToken 校验 HTTP 可用的 access/bot token。
func VerifyToken(tokenStr string) (*pkg.Claims, pkg.ErrCode) {
	return verifyToken(tokenStr, pkg.AccessTokenType, pkg.BotTokenType)
}

// VerifyWSTicket 校验 WebSocket 短时 ticket。
func VerifyWSTicket(tokenStr string) (*pkg.Claims, pkg.ErrCode) {
	claims, code := verifyToken(tokenStr, pkg.WSTicketType)
	if code != pkg.SUCCESS {
		return nil, code
	}
	if pkg.WSTicketExpired(claims) {
		return nil, pkg.TOKEN_EXPIRED
	}
	if redis.IsBlacklisted("ws:" + claims.UserUUID) {
		return nil, pkg.TOKEN_REVOKED
	}
	return claims, pkg.SUCCESS
}

func verifyToken(tokenStr string, allowedTypes ...string) (*pkg.Claims, pkg.ErrCode) {
	claims, err := pkg.ParseToken(tokenStr)
	if err != nil {
		return nil, pkg.TOKEN_WRONG
	}
	if !tokenTypeAllowed(claims, allowedTypes...) {
		return nil, pkg.TOKEN_WRONG
	}
	if claims.UserUUID == "" {
		return nil, pkg.TOKEN_WRONG
	}
	if pkg.IsTokenExpired(claims) {
		return nil, pkg.TOKEN_EXPIRED
	}
	if redis.IsBlacklisted(claims.ID) {
		return nil, pkg.TOKEN_REVOKED
	}
	if tokenVersionCheck != nil && claims.UserUUID != "" {
		currentVersion, err := cachedTokenVersion(claims.UserUUID)
		if err != nil {
			return nil, pkg.INTERNAL_ERROR
		}
		if currentVersion != claims.TokenVersion {
			return nil, pkg.TOKEN_REVOKED
		}
	}
	// Bot token 除 TokenVersion 外还必须通过 DB Revoked/ExpiresAt 校验。
	if claims.TokenType == pkg.BotTokenType && botTokenCheck != nil {
		valid, err := botTokenCheck.IsBotTokenValid(claims.UserUUID)
		if err != nil {
			return nil, pkg.INTERNAL_ERROR
		}
		if !valid {
			return nil, pkg.TOKEN_REVOKED
		}
	}
	return claims, pkg.SUCCESS
}

func tokenTypeAllowed(claims *pkg.Claims, allowedTypes ...string) bool {
	if claims == nil || claims.TokenType == "" {
		return false
	}
	for _, allowed := range allowedTypes {
		if claims.TokenType == allowed {
			return true
		}
	}
	return false
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
		// EventSource 无法自定义 Header，允许 HttpOnly cookie 作为统一鉴权兜底。
		if tokenHeader == "" {
			if cookie, err := c.Request.Cookie("gospeak_token"); err == nil {
				tokenHeader = cookie.Value
			}
		}
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
// 供 HTTP 中间件与 WebSocket Hub 共用，避免踢人/禁言路径口径不一致。
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
		allowed := "*"
		if cfg := config.Current(); cfg != nil && cfg.CORSOrigin != "" {
			allowed = cfg.CORSOrigin
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

// cachedTokenVersion 查询 TokenVersion，并做短 TTL 内存缓存避免每次请求都打 DB。
func cachedTokenVersion(userUUID string) (uint, error) {
	now := time.Now()
	tokenVersionCache.Lock()
	if entry, ok := tokenVersionCache.m[userUUID]; ok && now.Before(entry.expires) {
		tokenVersionCache.Unlock()
		return entry.version, nil
	}
	tokenVersionCache.Unlock()

	version, err := tokenVersionCheck.GetTokenVersionByUUID(userUUID)
	if err != nil {
		return 0, err
	}
	tokenVersionCache.Lock()
	tokenVersionCache.m[userUUID] = tokenVersionCacheEntry{version: version, expires: now.Add(tokenVersionCacheTTL)}
	tokenVersionCache.Unlock()
	return version, nil
}
