package middleware

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/policy"
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

// BlacklistChecker 校验 token 是否已吊销，nil 时按未黑名单放行（fail-open 语义）。
type BlacklistChecker func(key string) (bool, error)

// Dependencies 聚合 HTTP/WS 鉴权所需的业务实现，由组合根一次注入。
type Dependencies struct {
	PermissionChecker   permissionChecker
	TokenVersionChecker tokenVersionChecker
	DomainChecker       func(domainUUID, userUUID string) bool
	BlacklistChecker    BlacklistChecker
	AuthCookieName      string
}

// Auth 持有不可变依赖快照，生产路径由 Configure 一次注入。
type Auth struct {
	deps Dependencies
}

var authMu sync.RWMutex

var defaultAuth = &Auth{}

// Configure 原子合并默认鉴权依赖（零值字段不覆盖既有注入），生产启动时调用一次。
// 与下方字段级 setter 共享同一写锁与写语义，是依赖注入的唯一两个入口：
// 生产走 Configure，测试缝走字段级 setter。
func Configure(deps Dependencies) {
	authMu.Lock()
	defer authMu.Unlock()
	if deps.PermissionChecker != nil {
		defaultAuth.deps.PermissionChecker = deps.PermissionChecker
	}
	if deps.TokenVersionChecker != nil {
		defaultAuth.deps.TokenVersionChecker = deps.TokenVersionChecker
	}
	if deps.DomainChecker != nil {
		defaultAuth.deps.DomainChecker = deps.DomainChecker
	}
	if deps.BlacklistChecker != nil {
		defaultAuth.deps.BlacklistChecker = deps.BlacklistChecker
	}
	if deps.AuthCookieName != "" {
		defaultAuth.deps.AuthCookieName = deps.AuthCookieName
	}
}

func currentAuth() *Auth {
	authMu.RLock()
	defer authMu.RUnlock()
	return defaultAuth
}

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

// SetPermissionChecker 注入权限查询实现（测试缝；生产路径走 Configure）。
func SetPermissionChecker(c permissionChecker) {
	authMu.Lock()
	defaultAuth.deps.PermissionChecker = c
	authMu.Unlock()
}

// SetTokenVersionChecker 注入 token 版本查询实现（测试缝；生产路径走 Configure）。
func SetTokenVersionChecker(tvc tokenVersionChecker) {
	authMu.Lock()
	defaultAuth.deps.TokenVersionChecker = tvc
	authMu.Unlock()
}

// VerifyToken 校验 HTTP 可用的 access token。
func VerifyToken(tokenStr string) (*pkg.Claims, pkg.ErrCode) {
	return verifyToken(tokenStr, pkg.AccessTokenType)
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
	a := currentAuth()
	if checker := a.deps.BlacklistChecker; checker != nil {
		if revoked, err := checker(claims.ID); err != nil {
			return nil, pkg.INTERNAL_ERROR
		} else if revoked {
			return nil, pkg.TOKEN_REVOKED
		}
	}
	if a.deps.TokenVersionChecker != nil && claims.UserUUID != "" {
		currentVersion, err := cachedTokenVersion(claims.UserUUID)
		if err != nil {
			return nil, pkg.INTERNAL_ERROR
		}
		if currentVersion != claims.TokenVersion {
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

// gin context key 常量：仅通过下方 accessor 读取，禁止在包外散落魔法字符串。
const (
	ctxKeyUsername    = "username"
	ctxKeyDisplayName = "display_name"
	ctxKeyUserUUID    = "user_uuid"
	ctxKeyRole        = "role"
	ctxKeyClaims      = "claims"
	ctxKeyAuthType    = "auth_type"
)

func setClaimsContext(c *gin.Context, claims *pkg.Claims) {
	c.Set(ctxKeyUsername, claims.Username)
	c.Set(ctxKeyDisplayName, claims.DisplayName)
	c.Set(ctxKeyUserUUID, claims.UserUUID)
	c.Set(ctxKeyRole, claims.Role)
	c.Set(ctxKeyClaims, claims)
	c.Set(ctxKeyAuthType, "jwt")
}

// CurrentUser 返回 JWTAuth 注入的 claims；未鉴权返回 nil。
// 这是 handler/middleware 读取请求身份的唯一入口。
func CurrentUser(c *gin.Context) *pkg.Claims {
	v, ok := c.Get(ctxKeyClaims)
	if !ok {
		return nil
	}
	claims, _ := v.(*pkg.Claims)
	return claims
}

// 以下字段级 accessor 优先读 claims；测试 mock 或轻量路径只注入单键时回退读取。

func CurrentUsername(c *gin.Context) string {
	if claims := CurrentUser(c); claims != nil {
		return claims.Username
	}
	return c.GetString(ctxKeyUsername)
}

func CurrentUserUUID(c *gin.Context) string {
	if claims := CurrentUser(c); claims != nil {
		return claims.UserUUID
	}
	return c.GetString(ctxKeyUserUUID)
}

func CurrentDisplayName(c *gin.Context) string {
	if claims := CurrentUser(c); claims != nil {
		return claims.DisplayName
	}
	return c.GetString(ctxKeyDisplayName)
}

func CurrentRole(c *gin.Context) string {
	if claims := CurrentUser(c); claims != nil {
		return claims.Role
	}
	return c.GetString(ctxKeyRole)
}

func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenHeader := c.Request.Header.Get("Authorization")
		// EventSource 无法自定义 Header，允许 HttpOnly cookie 作为统一鉴权兜底。
		if tokenHeader == "" {
			name := currentAuth().deps.AuthCookieName
			if name == "" {
				name = "gospeak_token"
			}
			if cookie, err := c.Request.Cookie(name); err == nil {
				tokenHeader = cookie.Value
			}
		}
		if tokenHeader == "" {
			pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
			c.Abort()
			return
		}

		// Bearer 前缀或直传 token 均可，TrimPrefix 无前缀时原样返回。
		tokenStr := strings.TrimPrefix(tokenHeader, "Bearer ")

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
		if CurrentUser(c) == nil {
			pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
			c.Abort()
			return
		}

		roleStr := CurrentRole(c)
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
	return PermissionGranted(CurrentUser(c), CurrentRole(c), permCode, currentAuth().deps.PermissionChecker)
}

// PermissionGranted 统一权限判定：role → checker 映射。
// 供 HTTP 中间件与 WebSocket Hub 共用，避免踢人/禁言路径口径不一致。
func PermissionGranted(_ *pkg.Claims, role string, permCode string, checker permissionChecker) bool {
	return checker != nil && checker.HasPermission(role, permCode)
}

// RequirePermission 基于权限码的鉴权，替代硬编码角色名。
func RequirePermission(permCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if CurrentUser(c) == nil {
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

		if owner, ok := c.Get(ownerContextKey); ok && owner == CurrentUsername(c) && owner != "" {
			c.Next()
			return
		}

		pkg.Fail(c, pkg.FORBIDDEN)
		c.Abort()
	}
}

// BanCheck 复用传输无关的 policy.CheckBan，与 WS 建连/分发使用同一份封禁判决。
// roleCache 未初始化时 policy 层的字面量兜底仍能拦住 ban 角色，避免 fail-open。
func BanCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		sub := policy.Subject{Role: CurrentRole(c)}
		if d := policy.CheckBan(sub); !d.Allowed {
			pkg.Fail(c, d.Code)
			c.Abort()
			return
		}
		c.Next()
	}
}

// CORS 返回基于显式白名单的 CORS 中间件。
// origins 为逗号拆分后的原始配置项，"*" 表示放行任意来源；
// 始终设置 Vary: Origin，未命中白名单时不回显 Access-Control-Allow-Origin。
func CORS(origins []string) gin.HandlerFunc {
	allowed := make([]string, 0, len(origins))
	for _, o := range origins {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		allowed = append(allowed, strings.TrimRight(o, "/"))
	}
	return func(c *gin.Context) {
		c.Header("Vary", "Origin")

		origin := strings.TrimRight(strings.TrimSpace(c.Request.Header.Get("Origin")), "/")
		allowAll := false
		matched := ""
		for _, o := range allowed {
			if o == "*" {
				allowAll = true
				break
			}
			if origin != "" && strings.EqualFold(o, origin) {
				matched = o
				break
			}
		}

		if allowAll {
			c.Header("Access-Control-Allow-Origin", "*")
		} else if matched != "" {
			c.Header("Access-Control-Allow-Origin", origin)
		}

		if matched != "" || allowAll {
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
			c.Header("Access-Control-Max-Age", "86400")
		}
		// Cookie 鉴权需要显式携带凭据；通配来源与 credentials 互斥，只对白名单来源开启。
		if matched != "" {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

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
	// 惰性清理：读路径顺手淘汰过期条目，避免无界增长（替代后台 ticker goroutine）。
	for k, entry := range tokenVersionCache.m {
		if now.After(entry.expires) {
			delete(tokenVersionCache.m, k)
		}
	}
	tokenVersionCache.Unlock()

	version, err := currentAuth().deps.TokenVersionChecker.GetTokenVersionByUUID(userUUID)
	if err != nil {
		return 0, err
	}
	tokenVersionCache.Lock()
	tokenVersionCache.m[userUUID] = tokenVersionCacheEntry{version: version, expires: now.Add(tokenVersionCacheTTL)}
	tokenVersionCache.Unlock()
	return version, nil
}
