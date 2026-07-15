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

	deliverer atomic.Value // stores Deliverer

	fallbackFromExternal bool
	remoteHook           atomic.Value // stores func(event string, payload interface{})

	subs []*nats.Subscription
	mu   sync.Mutex

	closed atomic.Bool
	done   chan struct{}
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
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			log.Printf("[EventBus] nats connection closed")
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	b.nc = nc

	if err := b.subscribeAll(); err != nil {
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
	_ = ctx
	b.deliverLocal(event, payload)
	env, err := NewEnvelope(b.instanceID, "namespace", "", event, payload)
	if err != nil {
		return err
	}
	return b.publish(NamespaceSubject(b.prefix), env)
}

func (b *NATSBus) PublishRoom(ctx context.Context, room, event string, payload interface{}) error {
	_ = ctx
	b.deliverLocalRoom(room, event, payload)
	env, err := NewEnvelope(b.instanceID, "room", room, event, payload)
	if err != nil {
		return err
	}
	return b.publish(RoomSubject(b.prefix, room), env)
}

func (b *NATSBus) PublishInternal(ctx context.Context, event string, payload interface{}) error {
	_ = ctx
	env, err := NewEnvelope(b.instanceID, "internal", "", event, payload)
	if err != nil {
		return err
	}
	return b.publish(InternalSubject(b.prefix), env)
}


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
	close(b.done)
	return nil
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
	var env Envelope
	if err := json.Unmarshal(m.Data, &env); err != nil {
		log.Printf("[EventBus] bad envelope: %v", err)
		return
	}
	if env.InstanceID == b.instanceID {
		return
	}

	payload := decodePayload(env.Payload)

	// internal 事件不进 Socket.IO，只走 RemoteHook（缓存失效等）。
	if env.Scope != "internal" {
		d, ok := b.deliverer.Load().(Deliverer)
		if ok && d != nil {
			switch env.Scope {
			case "room":
				d.BroadcastToRoom(env.Room, env.Event, payload)
			default:
				d.BroadcastToNamespace(env.Event, payload)
			}
		}
	}

	if hook, ok := b.remoteHook.Load().(func(event string, payload interface{})); ok && hook != nil {
		hook(env.Event, payload)
	}
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

func (b *NATSBus) publish(subject string, env Envelope) error {
	if b.closed.Load() || b.nc == nil || !b.nc.IsConnected() {
		if b.fallbackFromExternal {
			return nil
		}
		log.Printf("[EventBus] skip nats publish (disconnected): %s %s", env.Scope, env.Event)
		return nil
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
