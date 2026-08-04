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
