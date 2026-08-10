package bus

import (
	"context"
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
	// WALPath 非空时启用断线事件磁盘持久化；为空保持纯内存行为（默认）。
	WALPath string
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

	wal *pendingWAL

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
	if cfg.WALPath != "" {
		wal, err := newPendingWAL(cfg.WALPath)
		if err != nil {
			return nil, fmt.Errorf("open pending wal: %w", err)
		}
		b.wal = wal
		if recovered, err := wal.ReadAll(); err == nil && len(recovered) > 0 {
			b.pending = recovered
			log.Printf("[EventBus] recovered %d pending events from WAL", len(recovered))
		}
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
	if b.wal != nil {
		_ = b.wal.Close()
	}
	b.deliverMu.Lock()
	if b.deliverCh != nil {
		close(b.deliverCh)
	}
	b.deliverMu.Unlock()
	close(b.done)
	return nil
}
