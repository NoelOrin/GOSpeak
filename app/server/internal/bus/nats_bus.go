package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	Deliverer     Deliverer
	// Mode 强制指定 "embedded" 或 "external"；为空时由 URL 自动判断。
	Mode string
	// FallbackFromExternal 为 true 表示此 bus 由外部不可用降级而来。
	FallbackFromExternal bool
}

// NATSBus 基于 NATS 的 EventBus 实现。
// 发布时先本地投递再 NATS pub（双写到 peer），收到 NATS 消息后按 InstanceID 去重。
type NATSBus struct {
	instanceID string
	prefix     string
	mode       string
	url        string

	nc *nats.Conn

	deliverer atomic.Value // stores Deliverer

	// fallbackFromExternal 为 true 表示此 bus 为嵌入 NATS 的降级模式，
	// NATS 不可用时仅做本地投递不报错。
	fallbackFromExternal bool

	subs []*nats.Subscription
	mu   sync.Mutex

	connected atomic.Bool
	cancel    context.CancelFunc
	done      chan struct{}
}

// NewNATSBus 创建并启动 NATSBus。
// 连接 NATS、订阅 namespace 和 room 主题，设置本地投递接口。
func NewNATSBus(cfg NATSBusConfig) (*NATSBus, error) {
	nc, err := nats.Connect(cfg.URL, nats.Timeout(5*time.Second))
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}

	_ , cancel := context.WithCancel(context.Background())

	b := &NATSBus{
		instanceID: cfg.InstanceID,
		prefix:     cfg.SubjectPrefix,
		url:        cfg.URL,
		nc:         nc,
		cancel:     cancel,
		done:       make(chan struct{}),
	}
	b.connected.Store(true)

	// Mode: cfg.Mode 优先，否则由 URL 自动判断
	b.fallbackFromExternal = cfg.FallbackFromExternal
	if cfg.Mode != "" {
		b.mode = cfg.Mode
	} else if strings.Contains(cfg.URL, "127.0.0.1:") || strings.Contains(cfg.URL, "localhost:") {
		b.mode = "embedded"
	} else {
		b.mode = "external"
	}

	if cfg.Deliverer != nil {
		b.deliverer.Store(cfg.Deliverer)
	}

	if err := b.subscribeAll(); err != nil {
		nc.Close()
		cancel()
		return nil, err
	}

	return b, nil
}

// Mode 返回 "embedded" 或 "external"。
func (b *NATSBus) Mode() string { return b.mode }

// IsConnected 返回 NATS 连接状态。
func (b *NATSBus) IsConnected() bool { return b.connected.Load() }

// InstanceID 返回此 bus 实例的唯一标识。
func (b *NATSBus) InstanceID() string { return b.instanceID }

// SetDeliverer 设置本地投递接口，可在启动后替换。
func (b *NATSBus) SetDeliverer(d Deliverer) {
	if d != nil {
		b.deliverer.Store(d)
	}
}

// ---------------------------------------------------------------------------
// EventBus 接口
// ---------------------------------------------------------------------------

func (b *NATSBus) PublishNamespace(ctx context.Context, event string, payload interface{}) error {
	// 1. 本地投递（使用原始 payload）
	b.deliverLocal(event, payload)

	// 2. NATS 发布
	env, err := NewEnvelope(b.instanceID, "namespace", "", event, payload)
	if err != nil {
		return err
	}
	return b.publish(ctx, NamespaceSubject(b.prefix), env)
}

func (b *NATSBus) PublishRoom(ctx context.Context, room, event string, payload interface{}) error {
	// 1. 本地投递到 room
	b.deliverLocalRoom(room, event, payload)

	// 2. NATS 发布
	env, err := NewEnvelope(b.instanceID, "room", room, event, payload)
	if err != nil {
		return err
	}
	return b.publish(ctx, RoomSubject(b.prefix, room), env)
}

func (b *NATSBus) Close() error {
	b.cancel()

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

	b.connected.Store(false)
	close(b.done)
	return nil
}

// ---------------------------------------------------------------------------
// 内部
// ---------------------------------------------------------------------------

// subscribeAll 订阅 namespace 和 room 通配主题。
func (b *NATSBus) subscribeAll() error {
	subNS, err := b.nc.Subscribe(NamespaceSubject(b.prefix), b.onMessage)
	if err != nil {
		return fmt.Errorf("subscribe namespace: %w", err)
	}

	subRoom, err := b.nc.Subscribe(RoomSubject(b.prefix, ">"), b.onMessage)
	if err != nil {
		_ = subNS.Unsubscribe()
		return fmt.Errorf("subscribe room wildcard: %w", err)
	}

	b.mu.Lock()
	b.subs = append(b.subs, subNS, subRoom)
	b.mu.Unlock()
	return nil
}

// onMessage 处理 NATS 消息：
//   - 反序列化为 Envelope
//   - 若 InstanceID 匹配自身则跳过（自去重）
//   - 按 scope 投递给本地 Deliverer
func (b *NATSBus) onMessage(m *nats.Msg) {
	var env Envelope
	if err := json.Unmarshal(m.Data, &env); err != nil {
		return
	}

	// 自去重：不处理自己发布的消息
	if env.InstanceID == b.instanceID {
		return
	}

	d, ok := b.deliverer.Load().(Deliverer)
	if !ok || d == nil {
		return
	}

	switch env.Scope {
	case "namespace":
		d.BroadcastToNamespace(env.Event, env.Payload)
	case "room":
		d.BroadcastToRoom(env.Room, env.Event, env.Payload)
	}
}

func (b *NATSBus) publish(_ context.Context, subject string, env Envelope) error {
	if !b.connected.Load() {
		if b.fallbackFromExternal {
			return nil // 降级：静默跳过
		}
		return fmt.Errorf("nats not connected")
	}

	data, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return b.nc.Publish(subject, data)
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
