package middleware

import (
	"bytes"
	"encoding/json"
	"io"

	"GOSpeak/internal/pkg"

	"github.com/gin-gonic/gin"
)

// domainChecker 在启动时通过 SetDomainChecker 注入，由 DomainService.IsMember 实现。
// 使用 RWMutex 保护，避免并发测试与生产请求之间的 data race。

// SetDomainChecker 注入 Domain 成员校验函数。
func SetDomainChecker(checker func(domainUUID, userUUID string) bool) {
	authMu.Lock()
	defaultAuth.deps.DomainChecker = checker
	authMu.Unlock()
}

// getDomainChecker 返回当前的 Domain 成员校验函数（并发安全）。
func getDomainChecker() func(domainUUID, userUUID string) bool {
	return currentAuth().deps.DomainChecker
}

// IsDomainMember 返回当前注入的 Domain 成员校验结果。未提供 domain_uuid 时放行；未接线 checker 时拒绝。
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

// RequireDomainMember 校验当前用户是否为指定 Domain 的成员。
// domain_uuid 从 URL、Query 或 JSON body 获取。
// RequireDomainMemberIfProvided 当请求携带 domain_uuid 时校验成员身份；未携带时放行，兼容平台级房间。
func RequireDomainMemberIfProvided() gin.HandlerFunc {
	return func(c *gin.Context) {
		domainUUID := c.Param("domain_uuid")
		if domainUUID == "" {
			domainUUID = c.Query("domain_uuid")
		}
		if domainUUID == "" {
			var body struct {
				DomainUUID string `json:"domain_uuid"`
			}
			raw, readErr := io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewReader(raw))
			if readErr == nil && len(raw) > 0 {
				if err := json.Unmarshal(raw, &body); err == nil {
					domainUUID = body.DomainUUID
				}
			}
		}
		if domainUUID == "" {
			c.Next()
			return
		}
		c.Set("domain_uuid", domainUUID)
		checker := getDomainChecker()
		if checker == nil {
			pkg.Fail(c, pkg.INTERNAL_ERROR, "domain checker not configured")
			c.Abort()
			return
		}
		userUUIDVal, exists := c.Get("user_uuid")
		if !exists {
			pkg.Fail(c, pkg.TOKEN_NOT_EXIST, "user not authenticated")
			c.Abort()
			return
		}
		userUUID, ok := userUUIDVal.(string)
		if !ok {
			pkg.Fail(c, pkg.INTERNAL_ERROR, "invalid user_uuid type")
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
	return func(c *gin.Context) {
		domainUUID := c.Param("domain_uuid")
		if domainUUID == "" {
			domainUUID = c.Query("domain_uuid")
		}
		if domainUUID == "" {
			var body struct {
				DomainUUID string `json:"domain_uuid"`
			}
			raw, readErr := io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewReader(raw))
			if readErr == nil && len(raw) > 0 {
				if err := json.Unmarshal(raw, &body); err == nil {
					domainUUID = body.DomainUUID
				}
			}
		}
		if domainUUID == "" {
			pkg.Fail(c, pkg.INVALID_PARAMS, "domain_uuid is required")
			c.Abort()
			return
		}
		c.Set("domain_uuid", domainUUID)

		checker := getDomainChecker()
		if checker == nil {
			pkg.Fail(c, pkg.INTERNAL_ERROR, "domain checker not configured")
			c.Abort()
			return
		}

		userUUIDVal, exists := c.Get("user_uuid")
		if !exists {
			pkg.Fail(c, pkg.TOKEN_NOT_EXIST, "user not authenticated")
			c.Abort()
			return
		}
		userUUID, ok := userUUIDVal.(string)
		if !ok {
			pkg.Fail(c, pkg.INTERNAL_ERROR, "invalid user_uuid type")
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
