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
	IsGuestBanned(domainUUID, userUUID string) bool
}

// guestAllowPrefixes 访客可调用的业务路径前缀：信令、房间（含房间消息）、
// 自助资料、域公开读接口与登出/刷新。
var guestAllowPrefixes = []string{
	"/api/v1/signal/",
	"/api/v1/room/",
	"/api/v1/user/profile",
	"/api/v1/auth/logout",
	"/api/v1/auth/refresh",
	"/api/v1/domain/get",
	"/api/v1/domain/members",
	"/api/v1/domain/list-public",
	"/api/v1/domain/preview",
}

// GuestGuardWith 返回使用指定 checker 的访客守卫；测试注入用。
func GuestGuardWith(checker guestChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		userUUID := contextUserUUID(c)
		if userUUID == "" || checker == nil || !checker.IsGuest(userUUID) {
			c.Next()
			return
		}
		if dom := guestDomainOf(c); dom != "" && checker.IsGuestBanned(dom, userUUID) {
			pkg.Fail(c, pkg.FORBIDDEN, "guest has been banned")
			c.Abort()
			return
		}
		if !guestPathAllowed(c.Request.URL.Path) {
			pkg.Fail(c, pkg.FORBIDDEN, "guest access not allowed")
			c.Abort()
			return
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
	var body struct {
		DomainUUID string `json:"domain_uuid"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return ""
	}
	return body.DomainUUID
}

func guestPathAllowed(path string) bool {
	for _, prefix := range guestAllowPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
