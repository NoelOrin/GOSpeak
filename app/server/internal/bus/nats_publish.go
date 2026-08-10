package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

func (b *NATSBus) publish(ctx context.Context, subject string, env Envelope) error {
	// fallback 静默模式优先于 ctx 取消检查：降级实例不发布，取消也不会计数。
	if b.fallbackFromExternal {
		return nil
	}
	if err := ctx.Err(); err != nil {
		b.droppedPublish.Add(1)
		log.Printf("[EventBus] publish canceled: %s %s: %v", env.Scope, env.Event, err)
		return err
	}
	if b.closed.Load() || b.nc == nil || !b.nc.IsConnected() {
		// 入口检查之后、入队之前存在竞态窗口，此处为 TOCTOU 兜底，非死代码。
		if err := ctx.Err(); err != nil {
			b.droppedPublish.Add(1)
			log.Printf("[EventBus] publish canceled: %s %s: %v", env.Scope, env.Event, err)
			return err
		}
		b.droppedPublish.Add(1)
		b.enqueuePending(subject, env)
		err := fmt.Errorf("nats publish %s: disconnected", subject)
		log.Printf("[EventBus] queue nats publish (disconnected): %s %s", env.Scope, env.Event)
		return err
	}
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}
	if err := b.nc.Publish(subject, data); err != nil {
		return fmt.Errorf("nats publish %s: %w", subject, err)
	}
	return nil
}

func (b *NATSBus) deliverLocal(event string, payload interface{}) {
	d, ok := b.deliverer.Load().(Deliverer)
	if !ok || d == nil {
		return
	}
	d.BroadcastToNamespace(event, payload)
}

func (b *NATSBus) deliverLocalRoom(room, event string, payload interface{}) {
	d, ok := b.deliverer.Load().(Deliverer)
	if !ok || d == nil {
		return
	}
	d.BroadcastToRoom(room, event, payload)
}

// enqueuePending 缓存断线期间的跨实例事件；超出容量时丢弃并继续计数。
func (b *NATSBus) enqueuePending(subject string, env Envelope) {
	b.pendingMu.Lock()
	defer b.pendingMu.Unlock()
	if len(b.pending) >= maxPendingPublish {
		b.droppedPublish.Add(1)
		log.Printf("[EventBus] pending publish queue full, dropping: %s %s", env.Scope, env.Event)
		return
	}
	b.pending = append(b.pending, pendingEnvelope{subject: subject, env: env})
	if b.wal != nil {
		if err := b.wal.Append(subject, env); err != nil {
			log.Printf("[EventBus] wal append failed, keeping in memory only: %v", err)
		}
	}
}

// flushPending 在重连后重放断线期间缓存的事件。重放失败不再入队，
// 避免连接反复抖动导致无限积压；最终一致性由共享 KV 兜底。
func (b *NATSBus) flushPending() {
	b.pendingMu.Lock()
	pending := b.pending
	b.pending = nil
	b.pendingMu.Unlock()
	for _, p := range pending {
		data, err := json.Marshal(p.env)
		if err != nil {
			continue
		}
		var publishErr error
		for attempt := 0; attempt < 3; attempt++ {
			if err := b.nc.Publish(p.subject, data); err == nil {
				publishErr = nil
				break
			} else {
				publishErr = err
				time.Sleep(200 * time.Millisecond)
			}
		}
		if publishErr != nil {
			b.droppedPublish.Add(1)
			log.Printf("[EventBus] pending publish replay failed: %s %s: %v", p.env.Scope, p.env.Event, publishErr)
		}
	}
	if b.wal != nil {
		if err := b.wal.Truncate(); err != nil {
			log.Printf("[EventBus] wal truncate failed: %v", err)
		}
	}
}
