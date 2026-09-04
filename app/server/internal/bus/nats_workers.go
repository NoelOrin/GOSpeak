package bus

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/nats-io/nats.go"
)

// startDeliverWorkers 启动 4 个并发消费同一有界队列的 worker。
// 跨事件投递顺序不保证；RemoteHook 也在 worker 内异步执行，
// 调用方不应依赖 onMessage 返回前 hook 已完成。
func (b *NATSBus) startDeliverWorkers() {
	b.deliverCh = make(chan queuedEnvelope, maxDeliverQueue)
	for i := 0; i < 4; i++ {
		go func() {
			for env := range b.deliverCh {
				if env.scope != "internal" {
					if d, ok := b.deliverer.Load().(Deliverer); ok && d != nil {
						if env.scope == "room" {
							d.BroadcastToRoom(env.room, env.event, env.payload)
						} else {
							d.BroadcastToNamespace(env.event, env.payload)
						}
					}
				}
				if env.hook != nil {
					env.hook(env.event, env.payload)
				}
			}
		}()
	}
}

func (b *NATSBus) subscribeAll() error {
	subNS, err := b.nc.Subscribe(NamespaceSubject(b.prefix), b.onMessage)
	if err != nil {
		return fmt.Errorf("subscribe namespace: %w", err)
	}
	// 明确通配，避免 RoomSubject(prefix, ">") 语义含糊
	roomSubject := b.prefix + ".signal.room.>"
	subRoom, err := b.nc.Subscribe(roomSubject, b.onMessage)
	if err != nil {
		_ = subNS.Unsubscribe()
		return fmt.Errorf("subscribe room wildcard: %w", err)
	}
	subInt, err := b.nc.Subscribe(InternalSubject(b.prefix), b.onMessage)
	if err != nil {
		_ = subNS.Unsubscribe()
		_ = subRoom.Unsubscribe()
		return fmt.Errorf("subscribe internal: %w", err)
	}
	if err := b.nc.Flush(); err != nil {
		_ = subNS.Unsubscribe()
		_ = subRoom.Unsubscribe()
		_ = subInt.Unsubscribe()
		return fmt.Errorf("subscribe flush: %w", err)
	}
	b.mu.Lock()
	b.subs = append(b.subs, subNS, subRoom, subInt)
	b.mu.Unlock()
	return nil
}

func (b *NATSBus) onMessage(m *nats.Msg) {
	if b.closed.Load() {
		return
	}
	var env Envelope
	if err := json.Unmarshal(m.Data, &env); err != nil {
		log.Printf("[EventBus] bad envelope: %v", err)
		return
	}
	if env.InstanceID == b.instanceID {
		return
	}

	payload := decodePayload(env.Payload)

	// 只做解包入队，实际投递由异步 worker 完成，避免阻塞 NATS 回调。
	b.deliverMu.Lock()
	if b.closed.Load() {
		b.deliverMu.Unlock()
		return
	}
	select {
	case b.deliverCh <- queuedEnvelope{
		scope:   env.Scope,
		room:    env.Room,
		event:   env.Event,
		payload: payload,
		hook: func(event string, payload interface{}) {
			if hook, ok := b.remoteHook.Load().(func(event string, payload interface{})); ok && hook != nil {
				hook(event, payload)
			}
		},
	}:
	default:
		b.droppedPublish.Add(1)
		log.Printf("[EventBus] deliver queue full, dropping: %s %s", env.Scope, env.Event)
	}
	b.deliverMu.Unlock()
}

func decodePayload(raw json.RawMessage) interface{} {
	if len(raw) == 0 {
		return nil
	}
	var payload interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return json.RawMessage(append([]byte(nil), raw...))
	}
	return payload
}
