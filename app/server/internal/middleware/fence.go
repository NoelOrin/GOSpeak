package middleware

import (
	"GOSpeak/internal/pkg"

	"github.com/gin-gonic/gin"
)

// FenceChecker 校验当前进程仍持有 Agent 写面；失败时请求必须被拒绝。
type FenceChecker func() error

// RequireAgentFence 对写方法执行 DB fence 校验，防止旧 Agent 在网络分区恢复后继续写。
func RequireAgentFence(check FenceChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case "POST", "PUT", "PATCH", "DELETE":
		default:
			c.Next()
			return
		}
		if check == nil {
			c.Next()
			return
		}
		if err := check(); err != nil {
			pkg.Fail(c, pkg.FORBIDDEN, "agent write fence lost")
			c.Abort()
			return
		}
		c.Next()
	}
}
