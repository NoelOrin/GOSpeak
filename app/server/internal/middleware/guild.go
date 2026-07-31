package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"

	"GOSpeak/internal/pkg"

	"github.com/gin-gonic/gin"
)

// guildChecker 在启动时通过 SetGuildChecker 注入，由 GuildService.IsMember 实现。
// 使用 RWMutex 保护，避免并发测试与生产请求之间的 data race。
var (
	guildCheckerMu sync.RWMutex
	guildChecker   func(guildUUID, userUUID string) bool
)

// SetGuildChecker 注入 Guild 成员校验函数。
func SetGuildChecker(checker func(guildUUID, userUUID string) bool) {
	guildCheckerMu.Lock()
	defer guildCheckerMu.Unlock()
	guildChecker = checker
}

// getGuildChecker 返回当前的 Guild 成员校验函数（并发安全）。
func getGuildChecker() func(guildUUID, userUUID string) bool {
	guildCheckerMu.RLock()
	defer guildCheckerMu.RUnlock()
	return guildChecker
}

// RequireGuildMember 校验当前用户是否为指定 Guild 的成员。
// guild_uuid 从 URL、Query 或 JSON body 获取；兼容历史接口使用的 uuid 字段。
func RequireGuildMember() gin.HandlerFunc {
	return func(c *gin.Context) {
		guildUUID := c.Param("guild_uuid")
		if guildUUID == "" {
			guildUUID = c.Query("guild_uuid")
		}
		if guildUUID == "" {
			guildUUID = c.Param("uuid")
		}
		if guildUUID == "" {
			guildUUID = c.Query("uuid")
		}
		if guildUUID == "" {
			var body struct {
				GuildUUID string `json:"guild_uuid"`
				UUID      string `json:"uuid"`
			}
			raw, readErr := io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewReader(raw))
			if readErr == nil && len(raw) > 0 {
				if err := json.Unmarshal(raw, &body); err == nil {
					if body.GuildUUID != "" {
						guildUUID = body.GuildUUID
					} else {
						guildUUID = body.UUID
					}
				}
			}
		}
		if guildUUID == "" {
			pkg.Fail(c, pkg.INVALID_PARAMS, "guild_uuid is required")
			c.Abort()
			return
		}
		c.Set("guild_uuid", guildUUID)

		checker := getGuildChecker()
		if checker == nil {
			pkg.Fail(c, pkg.INTERNAL_ERROR, "guild checker not configured")
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

		if !checker(guildUUID, userUUID) {
			pkg.Fail(c, pkg.FORBIDDEN, "not a member of this guild")
			c.Abort()
			return
		}
		c.Next()
	}
}
