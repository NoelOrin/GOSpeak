package cluster

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"GOSpeak/internal/logger"

	"github.com/nats-io/nats.go"
)

// LeaderLock 是 Phase 6 Agent 主备选举的 NATS KV 锁。
type LeaderLock interface {
	TryAcquire(ctx context.Context, nodeID string) (bool, error)
}

// NATSLeaderLock 使用 NATS KV 原子创建实现单写者锁。
type NATSLeaderLock struct {
	kv nats.KeyValue
}

// OpenLeaderLock 打开或创建主备锁 KV。
func OpenLeaderLock(js nats.JetStreamContext, prefix string) (*NATSLeaderLock, error) {
	bucket := prefix + "_leader"
	kv, err := js.KeyValue(bucket)
	if err != nil {
		if !errors.Is(err, nats.ErrBucketNotFound) {
			return nil, err
		}
		kv, err = js.CreateKeyValue(&nats.KeyValueConfig{
			Bucket: bucket,
			TTL:    5 * time.Second,
		})
		if err != nil {
			return nil, err
		}
	}
	return &NATSLeaderLock{kv: kv}, nil
}

// TryAcquire 尝试抢占 leader 键；键已存在时返回 false。
func (l *NATSLeaderLock) TryAcquire(_ context.Context, nodeID string) (bool, error) {
	_, err := l.kv.Create("active", []byte(nodeID))
	if err == nats.ErrKeyExists {
		return false, nil
	}
	if err != nil {
		msg := err.Error()
		if errors.Is(err, nats.ErrKeyExists) || strings.Contains(msg, "wrong last sequence") || strings.Contains(msg, "key exists") {
			return false, nil
		}
		return false, err
	}
	return err == nil, err
}

// RenewLoop 每 interval 更新锁 TTL。返回 done channel（ctx 取消后退出）
// 与 lost channel（检测到锁不再由本节点持有时关闭）。
// 一旦出现丢失窗口，即使随后重新抢占成功也上报 lost，调用方必须停止控制面，
// 避免与新的 Agent leader 形成双写。
func (l *NATSLeaderLock) RenewLoop(ctx context.Context, nodeID string, interval time.Duration) (<-chan struct{}, <-chan struct{}) {
	// 默认 2s，小于锁 TTL 5s。
	if interval <= 0 {
		interval = 2 * time.Second
	}
	done := make(chan struct{})
	lost := make(chan struct{})
	var lostOnce sync.Once
	failures := 0
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				entry, err := l.kv.Get("active")
				if err != nil || string(entry.Value()) != nodeID {
					// 锁已丢失：不再尝试无谓抢占，直接上报。
					lostOnce.Do(func() { close(lost) })
					continue
				}
				if _, err := l.kv.Update("active", []byte(nodeID), entry.Revision()); err != nil {
					// 更新失败不致命：下一 tick 的 Get 会重新校验归属；连续失败直至锁 TTL 到期才会被动发现丢锁。
					// 分区期可能持续失败，仅首次与每 10 次记录一次，避免按 renew interval 刷日志。
					failures++
					if failures == 1 || failures%10 == 0 {
						logger.WithComponent("Cluster").Warnf("leader lock renew failed count=%d node=%s: %v", failures, nodeID, err)
					}
				} else {
					failures = 0
				}
			}
		}
	}()
	return done, lost
}

// Release 显式释放锁；仅当当前持有者是 nodeID 时删除。
func (l *NATSLeaderLock) Release(nodeID string) error {
	entry, err := l.kv.Get("active")
	if err != nil {
		if errors.Is(err, nats.ErrKeyNotFound) {
			return nil
		}
		return err
	}
	if string(entry.Value()) != nodeID {
		return nil
	}
	return l.kv.Delete("active", nats.LastRevision(entry.Revision()))
}
