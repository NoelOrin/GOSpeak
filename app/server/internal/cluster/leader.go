package cluster

import (
	"context"
	"errors"
	"strings"
	"time"

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

// RenewLoop 每 interval 更新锁 TTL；返回 done channel，ctx 取消后退出。
func (l *NATSLeaderLock) RenewLoop(ctx context.Context, nodeID string, interval time.Duration) <-chan struct{} {
	// 默认 2s，小于锁 TTL 5s。
	if interval <= 0 {
		interval = 2 * time.Second
	}
	done := make(chan struct{})
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
					// 锁已丢失：尝试重新抢占
					_, _ = l.kv.Create("active", []byte(nodeID))
					continue
				}
				_, _ = l.kv.Update("active", []byte(nodeID), entry.Revision())
			}
		}
	}()
	return done
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
