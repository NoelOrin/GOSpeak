package middleware

import (
	"GOSpeak/internal/pkg"

	"github.com/gin-gonic/gin"
)

// guildChecker 在启动时通过 SetGuildChecker 注入，由 GuildService.IsMember 实现。
var guildChecker func(guildUUID, userUUID string) bool

// SetGuildChecker 注入 Guild 成员校验函数。
func SetGuildChecker(checker func(guildUUID, userUUID string) bool) {
	guildChecker = checker
}

// RequireGuildMember 校验当前用户是否为指定 Guild 的成员。
// guild_uuid 从以下来源依次获取：URL 参数、Query 参数、JSON body。
func RequireGuildMember() gin.HandlerFunc {
	return func(c *gin.Context) {
		guildUUID := c.Param("guild_uuid")
		if guildUUID == "" {
			guildUUID = c.Query("guild_uuid")
		}
		if guildUUID == "" {
			var body struct {
				GuildUUID string `json:"guild_uuid"`
			}
			if err := c.ShouldBindJSON(&body); err == nil && body.GuildUUID != "" {
				guildUUID = body.GuildUUID
			}
		}
		if guildUUID == "" {
			pkg.Fail(c, pkg.INVALID_PARAMS, "guild_uuid is required")
			c.Abort()
			return
		}
		c.Set("guild_uuid", guildUUID)

		if guildChecker == nil {
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

		if !guildChecker(guildUUID, userUUID) {
			pkg.Fail(c, pkg.FORBIDDEN, "not a member of this guild")
			c.Abort()
			return
		}
		c.Next()
	}
}
