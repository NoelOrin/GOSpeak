// Package jobs 提供后台定时任务。
package jobs

import (
	"context"
	"sync"
	"time"
)

// StartMuteExpiryScanner 周期调用 scan（如 MuteService.ListActiveMutes），
// 触发过期禁言的清理与 onExpired 回调（广播 member:unmuted + SFU 恢复）。
// 返回 stop 函数；stop 可重复调用，ctx 取消后停止。
func StartMuteExpiryScanner(ctx context.Context, scan func(), interval time.Duration) (stop func()) {
	stopCh := make(chan struct{})
	var stopOnce sync.Once
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stopCh:
				return
			case <-ticker.C:
				scan()
			}
		}
	}()
	return func() { stopOnce.Do(func() { close(stopCh) }) }
}
