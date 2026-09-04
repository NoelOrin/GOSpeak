package bus

import (
	"context"
	"encoding/json"
	"time"
)

const EnvelopeVersion = 1

type Envelope struct {
	V          int             `json:"v"`
	InstanceID string          `json:"instance_id"`
	Scope      string          `json:"scope"`
	Room       string          `json:"room,omitempty"`
	Event      string          `json:"event"`
	Payload    json.RawMessage `json:"payload"`
	TS         int64           `json:"ts"`
}

// Deliverer 将事件投递到本机 WebSocket。
type Deliverer interface {
	BroadcastToNamespace(event string, data interface{})
	BroadcastToRoom(room, event string, data interface{})
}

// EventBus 信号事件总线。
type EventBus interface {
	// PublishNamespace 先本地投递，再发布到 NATS（若可用）。
	PublishNamespace(ctx context.Context, event string, payload interface{}) error
	// PublishRoom 先本地投递到 room，再发布到 NATS。
	PublishRoom(ctx context.Context, room, event string, payload interface{}) error
	// PublishInternal 仅 NATS 发布（不经 WebSocket Deliverer），用于缓存失效等内部事件。
	PublishInternal(ctx context.Context, event string, payload interface{}) error
	Mode() string // "embedded" | "external"
	IsConnected() bool
	InstanceID() string
	Close() error
}

func NamespaceSubject(prefix string) string {
	return prefix + ".signal.namespace"
}

func RoomSubject(prefix, room string) string {
	return prefix + ".signal.room." + room
}

func InternalSubject(prefix string) string {
	return prefix + ".internal"
}

func NewEnvelope(instanceID, scope, room, event string, payload interface{}) (Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	if raw == nil {
		raw = []byte("null")
	}
	return Envelope{
		V:          EnvelopeVersion,
		InstanceID: instanceID,
		Scope:      scope,
		Room:       room,
		Event:      event,
		Payload:    raw,
		TS:         time.Now().UnixMilli(),
	}, nil
}
