package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"GOSpeak/internal/pkg"

	"github.com/gin-gonic/gin"
)

// guestChecker 抽象访客身份/封禁查询，由组合根注入（GuestService 适配）。
type guestChecker interface {
	IsGuest(userUUID string) bool
	IsGuestBanned(domainUUID, userUUID string) (bool, error)
	IsGuestDomainMember(domainUUID, userUUID string) (bool, error)
}

// 访客白名单只暴露信令、房间消息、自助会话与公开域列表。
var guestAllowExactPaths = map[string]struct{}{
	"/api/v1/user/profile":       {},
	"/api/v1/auth/logout":        {},
	"/api/v1/auth/refresh":       {},
	"/api/v1/auth/guest/renew":   {},
	"/api/v1/domain/list-public": {},
}

// guestDomainReadPaths 要求请求域必须是访客已加入的域。
var guestDomainReadPaths = map[string]struct{}{
	"/api/v1/domain/get":     {},
	"/api/v1/domain/members": {},
	"/api/v1/domain/preview": {},
}

// guestAllowSignalPaths 访客在信令组仅能签发进房 token；rooms/participants 等
// 枚举端点对访客关闭，避免枚举全平台房间/参与者造成跨域信息泄漏。
var guestAllowSignalPaths = map[string]struct{}{
	"/api/v1/signal/token": {},
}

// GuestGuardWith 返回使用指定 checker 的访客守卫；测试注入用。
func GuestGuardWith(checker guestChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		userUUID := contextUserUUID(c)
		if userUUID == "" || checker == nil || !checker.IsGuest(userUUID) {
			c.Next()
			return
		}
		dom := guestDomainOf(c)
		banned, err := checker.IsGuestBanned(dom, userUUID)
		if err != nil {
			pkg.Fail(c, pkg.INTERNAL_ERROR, "guest ban check failed")
			c.Abort()
			return
		}
		if banned {
			pkg.Fail(c, pkg.FORBIDDEN, "guest has been banned")
			c.Abort()
			return
		}
		if !guestPathAllowed(c.Request.URL.Path) {
			pkg.Fail(c, pkg.FORBIDDEN, "guest access not allowed")
			c.Abort()
			return
		}
		if _, scoped := guestDomainReadPaths[c.Request.URL.Path]; scoped {
			if dom == "" {
				pkg.Fail(c, pkg.FORBIDDEN, "guest access not allowed")
				c.Abort()
				return
			}
			member, err := checker.IsGuestDomainMember(dom, userUUID)
			if err != nil {
				pkg.Fail(c, pkg.INTERNAL_ERROR, "guest domain check failed")
				c.Abort()
				return
			}
			if !member {
				pkg.Fail(c, pkg.FORBIDDEN, "guest access not allowed")
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

// GuestGuard 使用全局注入的 guestChecker（生产路径，JWTAuth 之后挂载）。
func GuestGuard() gin.HandlerFunc {
	return GuestGuardWith(currentAuth().deps.GuestChecker)
}

func contextUserUUID(c *gin.Context) string {
	if v, ok := c.Get("user_uuid"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// guestDomainOf 依次从 query、header、JSON body 提取 domain_uuid；
// body 场景读取后回填，保证下游 ShouldBindJSON 不受影响。
func guestDomainOf(c *gin.Context) string {
	if dom := c.Query("domain_uuid"); dom != "" {
		return dom
	}
	if dom := c.GetHeader("X-Domain-UUID"); dom != "" {
		return dom
	}
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return ""
	}
	ct := c.GetHeader("Content-Type")
	if !strings.Contains(ct, "application/json") {
		return ""
	}
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))
	c.Request.ContentLength = int64(len(raw))
	var body struct {
		DomainUUID string `json:"domain_uuid"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return ""
	}
	return body.DomainUUID
}

func guestPathAllowed(path string) bool {
	// 文字消息组前缀放行；写操作由 guest_messaging_allowed 短路，读操作走域角色权限（fail-closed）。
	if strings.HasPrefix(path, "/api/v1/room/messages/") {
		return true
	}
	if _, ok := guestAllowExactPaths[path]; ok {
		return true
	}
	_, ok := guestAllowSignalPaths[path]
	return ok
}
