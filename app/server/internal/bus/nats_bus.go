package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
)

const maxPendingPublish = 1024

const maxDeliverQueue = 4096

// NATSBusConfig 构造 NATSBus 的参数。
type NATSBusConfig struct {
	InstanceID    string
	SubjectPrefix string
	URL           string
	Name          string
	Deliverer     Deliverer
	// Mode 必须为 "embedded" 或 "external"；为空时默认 "external"。
	Mode string
	// FallbackFromExternal 为 true 表示此 bus 由外部不可用降级而来。
	FallbackFromExternal bool
	// RemoteHook 在 peer 消息投递到本地 Deliverer 之后调用（不含自身回环）。
	// 用于 Hub 等需要同步处理控制事件（如 sfu:provider-changed）的场景。
	RemoteHook func(event string, payload interface{})
}

// NATSBus 基于 NATS 的 EventBus 实现。
// 发布时先本地投递再 NATS pub；收到 NATS 消息后按 InstanceID 去重。
type NATSBus struct {
	instanceID string
	prefix     string
	mode       string
	url        string

	nc *nats.Conn

	deliverCh chan queuedEnvelope

	deliverer atomic.Value // stores Deliverer

	fallbackFromExternal bool
	remoteHook           atomic.Value // stores func(event string, payload interface{})

	subs []*nats.Subscription
	mu   sync.Mutex

	closed atomic.Bool
	done   chan struct{}

	droppedPublish atomic.Uint64 // 发布/投递路径上的损失/告警事件计数（断线、取消、队列满、重放失败）

	deliverMu sync.Mutex // 保护 deliverCh 入队与 Close 关闭
	pendingMu sync.Mutex
	pending   []pendingEnvelope
}

// queuedEnvelope 是 onMessage 解包后等待 worker 投递的事件。
type queuedEnvelope struct {
	scope   string
	room    string
	event   string
	payload interface{}
	hook    func(event string, payload interface{})
}

// pendingEnvelope 是 NATS 断线期间暂存的事件，重连后重放。
type pendingEnvelope struct {
	subject string
	env     Envelope
}

// NewNATSBus 创建并启动 NATSBus。
func NewNATSBus(cfg NATSBusConfig) (*NATSBus, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("nats bus: empty URL")
	}
	if cfg.InstanceID == "" {
		return nil, fmt.Errorf("nats bus: empty InstanceID")
	}
	if cfg.SubjectPrefix == "" {
		cfg.SubjectPrefix = "gospeak"
	}
	mode := cfg.Mode
	if mode == "" {
		mode = "external"
	}
	if mode != "embedded" && mode != "external" {
		return nil, fmt.Errorf("nats bus: invalid mode %q", mode)
	}
	name := cfg.Name
	if name == "" {
		name = cfg.InstanceID
	}

	b := &NATSBus{
		instanceID:           cfg.InstanceID,
		prefix:               cfg.SubjectPrefix,
		mode:                 mode,
		url:                  cfg.URL,
		fallbackFromExternal: cfg.FallbackFromExternal,
		done:                 make(chan struct{}),
	}
	if cfg.Deliverer != nil {
		b.deliverer.Store(cfg.Deliverer)
	}
	if cfg.RemoteHook != nil {
		b.remoteHook.Store(cfg.RemoteHook)
	}

	nc, err := nats.Connect(cfg.URL,
		nats.Name(name),
		nats.Timeout(5*time.Second),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				log.Printf("[EventBus] nats disconnected: %v", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("[EventBus] nats reconnected: %s", nc.ConnectedUrl())
			b.flushPending()
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			log.Printf("[EventBus] nats connection closed")
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	b.nc = nc

	b.startDeliverWorkers()

	if err := b.subscribeAll(); err != nil {
		b.closed.Store(true)
		b.deliverMu.Lock()
		close(b.deliverCh)
		b.deliverMu.Unlock()
		nc.Close()
		return nil, err
	}
	return b, nil
}

func (b *NATSBus) Mode() string { return b.mode }

func (b *NATSBus) IsConnected() bool {
	return b.nc != nil && b.nc.IsConnected() && !b.closed.Load()
}

func (b *NATSBus) InstanceID() string { return b.instanceID }

// DroppedPublishCount 返回发布/投递路径上的损失/告警事件计数。
// 统计范围包括：NATS 断线未发出、ctx 已取消发布、本地投递队列满、
// pending 队列满、重放失败。同一事件可能在多个阶段重复计数，
// 因此该值应视为告警计数，而非精确的未投递事件数。
func (b *NATSBus) DroppedPublishCount() uint64 {
	return b.droppedPublish.Load()
}

// Conn returns the underlying NATS connection (for JetStream KV/jobs reuse).
func (b *NATSBus) Conn() *nats.Conn { return b.nc }

// SetDeliverer 设置本地投递接口，可在启动后替换。
func (b *NATSBus) SetDeliverer(d Deliverer) {
	if d != nil {
		b.deliverer.Store(d)
	}
}

// SetRemoteHook 设置 peer 事件钩子（投递后调用）。
func (b *NATSBus) SetRemoteHook(hook func(event string, payload interface{})) {
	if hook != nil {
		b.remoteHook.Store(hook)
	}
}

func (b *NATSBus) PublishNamespace(ctx context.Context, event string, payload interface{}) error {
	b.deliverLocal(event, payload)
	env, err := NewEnvelope(b.instanceID, "namespace", "", event, payload)
	if err != nil {
		return err
	}
	return b.publish(ctx, NamespaceSubject(b.prefix), env)
}

func (b *NATSBus) PublishRoom(ctx context.Context, room, event string, payload interface{}) error {
	b.deliverLocalRoom(room, event, payload)
	env, err := NewEnvelope(b.instanceID, "room", room, event, payload)
	if err != nil {
		return err
	}
	return b.publish(ctx, RoomSubject(b.prefix, room), env)
}

func (b *NATSBus) PublishInternal(ctx context.Context, event string, payload interface{}) error {
	env, err := NewEnvelope(b.instanceID, "internal", "", event, payload)
	if err != nil {
		return err
	}
	return b.publish(ctx, InternalSubject(b.prefix), env)
}

// Close 关闭 NATS 连接与投递队列后立即返回；worker 会继续消费已入队事件，
// 可能晚于 Close 完成投递，调用方不应假设 Close 返回后已无投递。
func (b *NATSBus) Close() error {
	if !b.closed.CompareAndSwap(false, true) {
		return nil
	}

	b.mu.Lock()
	subs := b.subs
	b.subs = nil
	b.mu.Unlock()

	for _, sub := range subs {
		_ = sub.Unsubscribe()
	}
	if b.nc != nil {
		b.nc.Close()
	}
	b.deliverMu.Lock()
	if b.deliverCh != nil {
		close(b.deliverCh)
	}
	b.deliverMu.Unlock()
	close(b.done)
	return nil
}

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
		if err := b.nc.Publish(p.subject, data); err != nil {
			b.droppedPublish.Add(1)
			log.Printf("[EventBus] pending publish replay failed: %s %s: %v", p.env.Scope, p.env.Event, err)
		}
	}
}
