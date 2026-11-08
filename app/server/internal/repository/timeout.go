package repository

import (
	"context"
	"time"
)

// repoTimeoutCtx 返回信号/请求路径统一使用的数据库操作超时，
// 避免慢 DB 卡住 WebSocket 读循环或锁内路径。
func repoTimeoutCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Second)
}
