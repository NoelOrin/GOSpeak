package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"

	"github.com/gin-gonic/gin"
)

// maxDomainBodyBytes limits body reading: the middleware only needs
// domain_uuid, so oversized requests are rejected before handlers run.
const maxDomainBodyBytes = 1 << 20 // 1 MiB

// SetDomainChecker 注入 Domain 成员判定（测试缝；生产路径走 Configure）。
func SetDomainChecker(checker func(domainUUID, userUUID string) bool) {
	authMu.Lock()
	defaultAuth.deps.DomainChecker = checker
	authMu.Unlock()
}

func getDomainChecker() func(domainUUID, userUUID string) bool {
	return currentAuth().deps.DomainChecker
}

// IsDomainMember returns the injected membership decision. Empty domains are
// platform-level resources and are allowed.
func IsDomainMember(domainUUID, userUUID string) bool {
	if domainUUID == "" {
		return true
	}
	checker := getDomainChecker()
	if checker == nil {
		return false
	}
	return checker(domainUUID, userUUID)
}

// IsPlatformAdmin identifies a human platform administrator. Bot tokens must
// present their own explicit permissions and never inherit the admin role.
func IsPlatformAdmin(c *gin.Context) bool {
	claims := CurrentUser(c)
	return claims != nil && claims.Role == model.RoleAdmin
}

func requestDomainUUID(c *gin.Context, required bool) (string, error) {
	domainUUID := c.Param("domain_uuid")
	if domainUUID == "" {
		domainUUID = c.Query("domain_uuid")
	}
	if domainUUID != "" {
		return domainUUID, nil
	}

	raw, err := snapshotRequestBody(c)
	if err != nil {
		return "", err
	}
	var body struct {
		DomainUUID string `json:"domain_uuid"`
	}
	if jsonErr := json.Unmarshal(raw, &body); jsonErr == nil {
		domainUUID = body.DomainUUID
	}
	return domainUUID, nil
}

// snapshotRequestBody 读取并回填请求体（有 1MiB 上限）。gin 的 ShouldBindBodyWith
// 只对链式 ShouldBindBodyWith 调用生效，后续 handler 的普通 ShouldBindJSON 仍会
// 重读 c.Request.Body，因此中间件探体必须手工回填 Body 与 ContentLength。
func snapshotRequestBody(c *gin.Context) ([]byte, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxDomainBodyBytes)
	raw, err := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))
	c.Request.ContentLength = int64(len(raw))
	return raw, err
}

func requireDomainMember(required, allowPlatformAdmin bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Room list treats an absent domain as platform scope.
		domainUUID, err := requestDomainUUID(c, required)
		if err != nil {
			pkg.Fail(c, pkg.INVALID_PARAMS, "request body too large")
			c.Abort()
			return
		}
		if !required && domainUUID == "" {
			c.Next()
			return
		}
		if required && domainUUID == "" {
			pkg.Fail(c, pkg.INVALID_PARAMS, "domain_uuid is required")
			c.Abort()
			return
		}
		c.Set("domain_uuid", domainUUID)

		userUUID := CurrentUserUUID(c)
		if userUUID == "" {
			pkg.Fail(c, pkg.TOKEN_NOT_EXIST, "user not authenticated")
			c.Abort()
			return
		}

		if allowPlatformAdmin && IsPlatformAdmin(c) {
			c.Next()
			return
		}

		checker := getDomainChecker()
		if checker == nil {
			pkg.Fail(c, pkg.INTERNAL_ERROR, "domain checker not configured")
			c.Abort()
			return
		}
		if !checker(domainUUID, userUUID) {
			pkg.Fail(c, pkg.FORBIDDEN, "not a member of this domain")
			c.Abort()
			return
		}
		c.Next()
	}
}

func RequireDomainMember() gin.HandlerFunc {
	return requireDomainMember(true, false)
}

func RequireDomainMemberIfProvided() gin.HandlerFunc {
	return requireDomainMember(false, false)
}

func RequirePlatformAdminOrDomainMember() gin.HandlerFunc {
	return requireDomainMember(true, true)
}

func RequirePlatformAdminOrDomainMemberIfProvided() gin.HandlerFunc {
	return requireDomainMember(false, true)
}
