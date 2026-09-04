package handler

import "github.com/gin-gonic/gin"

// auditActor 从 gin.Context 提取当前操作者身份，供审计日志使用。
// 返回 (userUUID, username)；上下文中缺失时返回空字符串。
func auditActor(c *gin.Context) (uuid, name string) {
	if v, ok := c.Get("user_uuid"); ok {
		if s, ok := v.(string); ok {
			uuid = s
		}
	}
	if v, ok := c.Get("username"); ok {
		if s, ok := v.(string); ok {
			name = s
		}
	}
	return
}
