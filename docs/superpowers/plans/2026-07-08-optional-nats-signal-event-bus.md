# 可选的 NATS 信号事件总线实现计划

> **对 agent 工作者：** 必选子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐个任务实施本计划。步骤使用复选框（`- [ ]`）语法跟踪。

**目标：** 为 GOSpeak 引入一个可选的 NATS 后端信号事件总线，使 Socket.IO 房间事件默认保持单节点运行，并在配置了 `NATS_URL` 时跨实例干净地复制。

**架构：** 房间成员、静音状态和 SRS 流状态暂时保持在进程内；仅将事件扇出路径提取到新的 `signal.EventBus` 接口后面。默认实现保持为 `socketio.Server` 上的一个薄本地适配器，而可选的 NATS 实现在本地投递并通过同节点去重将信封重新发布到其他节点。

**技术栈：** Go 1.26, Gin, go-socket.io, NATS Go 客户端 (`github.com/nats-io/nats.go`), Docker Compose, Go `testing`。

---

## 范围检查

本计划仅涵盖**一个子系统**：可选的基于 NATS 的**信号事件扇出**。

它**不会**在同一更改中尝试分发这些独立关注点：

- 房间成员状态 (`Hub.rooms`)
- SRS 流存在注册表 (`activeStreams`, `roomStreams`, `streamByIdentity`)
- SFU 提供者状态协调

如果需要，这些应该有单独的计划。本计划仍然可以独立地产生可工作、可测试的软件，因为单实例继续使用本地事件总线，而多实例部署获得复制的 `room:*`、`member:*` 和 `user:*` 广播。

## 文件结构

| 文件 | 操作 | 职责 |
|------|--------|----------------|
| `app/server/internal/signal/event_bus.go` | 创建 | 事件总线接口 + 共享 NATS 事件信封 |
| `app/server/internal/signal/local_event_bus.go` | 创建 | 包装 `socketServer` 命名空间/房间广播的本地总线 |
| `app/server/internal/signal/local_event_bus_test.go` | 创建 | 本地命名空间/房间发布的单元测试 |
| `app/server/internal/signal/event_bus_test_helpers_test.go` | 创建 | 共享测试替身：`recordingEventBus`、`fakeNATSClient` |
| `app/server/internal/signal/nats_event_bus.go` | 创建 | 可选复制总线：本地投递 + NATS 发布/订阅 |
| `app/server/internal/signal/nats_event_bus_test.go` | 创建 | 信封编码、远程投递、自消息忽略的单元测试 |
| `app/server/internal/signal/hub.go` | 修改 | 将直接的 `server.Broadcast*` 调用替换为 `EventBus` 辅助方法 |
| `app/server/internal/signal/hub_event_bus_test.go` | 创建 | 证明配置的事件总线被使用的 Hub 测试 |
| `app/server/internal/config/config.go` | 修改 | 添加 `NATSURL` 和 `NATSSubjectPrefix` 环境变量支持的设置 |
| `app/server/internal/signal/nats_client.go` | 创建 | 总线工厂使用的真实 NATS 客户端适配器 |
| `app/server/internal/signal/bus_factory.go` | 创建 | 根据配置构建本地或 NATS 后端总线 |
| `app/server/internal/signal/bus_factory_test.go` | 创建 | 配置驱动总线选择的测试 |
| `app/server/server/gin.go` | 修改 | 将事件总线接入 Hub 启动和优雅关闭 |
| `app/server/go.mod` | 修改 | 将 `github.com/nats-io/nats.go` 提升为直接依赖 |
| `app/server/go.sum` | 修改 | `go mod tidy` 后的依赖校验和更新 |
| `deploy/docker-compose.example.yml` | 修改 | 本地多实例测试的可选 NATS 服务 |
| `ARCHITECTURE.md` | 修改 | 记录新的可选事件总线和环境变量 |

---

### 任务 1：创建信号事件总线契约和本地实现

**文件：**
- 创建：`app/server/internal/signal/event_bus.go`
- 创建：`app/server/internal/signal/local_event_bus.go`
- 测试：`app/server/internal/signal/local_event_bus_test.go`

- [ ] **步骤 1：编写会失败的测试**

创建 `app/server/internal/signal/local_event_bus_test.go`：

```go
package signal

import "testing"

func TestLocalEventBus_PublishNamespace_BroadcastsToNamespace(t *testing.T) {
	server := newMockServer()
	bus := NewLocalEventBus(server)
	payload := map[string]interface{}{"room": "alpha"}

	if err := bus.PublishNamespace(EventRoomUpdated, payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := server.broadcasts[EventRoomUpdated]
	if len(got) != 1 {
		t.Fatalf("expected 1 namespace payload, got %d", len(got))
	}
	data, ok := got[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map payload, got %T", got[0])
	}
	if data["room"] != "alpha" {
		t.Fatalf("expected room alpha, got %#v", data["room"])
	}
}

func TestLocalEventBus_PublishRoom_BroadcastsToRoom(t *testing.T) {
	server := newMockServer()
	bus := NewLocalEventBus(server)
	payload := map[string]interface{}{"identity": "alice"}

	if err := bus.PublishRoom("alpha", EventMemberUpdated, payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roomEvents := server.roomCasts["alpha"]
	got := roomEvents[EventMemberUpdated]
	if len(got) != 1 {
		t.Fatalf("expected 1 room payload, got %d", len(got))
	}
	if got[0].(map[string]interface{})["identity"] != "alice" {
		t.Fatalf("expected identity alice, got %#v", got[0])
	}
}
```

- [ ] **步骤 2：运行测试确认其失败**

运行：`cd app/server && go test ./internal/signal -run TestLocalEventBus -v`

预期：FAIL，报 `undefined: NewLocalEventBus` 等

- [ ] **步骤 3：编写最小化实现**

创建 `app/server/internal/signal/event_bus.go`：

```go
package signal

const (
	EventScopeNamespace = "namespace"
	EventScopeRoom      = "room"
)

type EventEnvelope struct {
	NodeID string          `json:"node_id"`
	Scope  string          `json:"scope"`
	Room   string          `json:"room,omitempty"`
	Event  string          `json:"event"`
	Data   json.RawMessage `json:"data"`
}

type EventBus interface {
	PublishNamespace(event string, data interface{}) error
	PublishRoom(room, event string, data interface{}) error
	Close() error
}
```

创建 `app/server/internal/signal/local_event_bus.go`：

```go
package signal

type LocalEventBus struct {
	server socketServer
}

func NewLocalEventBus(server socketServer) *LocalEventBus {
	return &LocalEventBus{server: server}
}

func (b *LocalEventBus) PublishNamespace(event string, data interface{}) error {
	if b.server != nil {
		b.server.BroadcastToNamespace("/", event, data)
	}
	return nil
}

func (b *LocalEventBus) PublishRoom(room, event string, data interface{}) error {
	if b.server != nil {
		b.server.BroadcastToRoom("/", room, event, data)
	}
	return nil
}

func (b *LocalEventBus) Close() error {
	return nil
}
```

- [ ] **步骤 4：运行测试确认通过**

运行：`cd app/server && go test ./internal/signal -run TestLocalEventBus -v`

预期：PASS

- [ ] **步骤 5：提交**

```bash
git add app/server/internal/signal/event_bus.go \
        app/server/internal/signal/local_event_bus.go \
        app/server/internal/signal/local_event_bus_test.go
git commit -m "feat(signal): add event bus interface and local implementation"
```

---

### 任务 2：创建 NATS 事件总线实现

**文件：**
- 创建：`app/server/internal/signal/event_bus_test_helpers_test.go`
- 创建：`app/server/internal/signal/nats_event_bus.go`
- 测试：`app/server/internal/signal/nats_event_bus_test.go`

- [ ] **步骤 1：编写会失败的测试**

创建 `app/server/internal/signal/event_bus_test_helpers_test.go`：

```go
package signal

import "io"

type recordedNamespaceEvent struct {
	event string
	data  interface{}
}

type recordedRoomEvent struct {
	room  string
	event string
	data  interface{}
}

type recordingEventBus struct {
	namespace []recordedNamespaceEvent
	rooms     []recordedRoomEvent
}

func (b *recordingEventBus) PublishNamespace(event string, data interface{}) error {
	b.namespace = append(b.namespace, recordedNamespaceEvent{event: event, data: data})
	return nil
}

func (b *recordingEventBus) PublishRoom(room, event string, data interface{}) error {
	b.rooms = append(b.rooms, recordedRoomEvent{room: room, event: event, data: data})
	return nil
}

func (b *recordingEventBus) Close() error {
	return nil
}

type fakeSubscription struct {
	closeFn func() error
}

func (s *fakeSubscription) Close() error {
	if s.closeFn != nil {
		return s.closeFn()
	}
	return nil
}

type publishedMessage struct {
	subject string
	data    []byte
}

type fakeNATSClient struct {
	subscribers map[string]func(subject string, data []byte)
	published   []publishedMessage
	drained     bool
	closed      bool
}

func newFakeNATSClient() *fakeNATSClient {
	return &fakeNATSClient{
		subscribers: make(map[string]func(subject string, data []byte)),
		published:   make([]publishedMessage, 0),
	}
}

func (c *fakeNATSClient) Publish(subject string, data []byte) error {
	c.published = append(c.published, publishedMessage{subject: subject, data: data})
	if handler, ok := c.subscribers[subject]; ok {
		handler(subject, data)
	}
	return nil
}

func (c *fakeNATSClient) Subscribe(subject string, handler func(subject string, data []byte)) (io.Closer, error) {
	c.subscribers[subject] = handler
	return &fakeSubscription{
		closeFn: func() error {
			delete(c.subscribers, subject)
			return nil
		},
	}, nil
}

func (c *fakeNATSClient) Emit(subject string, data []byte) {
	if handler, ok := c.subscribers[subject]; ok {
		handler(subject, data)
	}
}

func (c *fakeNATSClient) Drain() error {
	c.drained = true
	return nil
}

func (c *fakeNATSClient) Close() error {
	c.closed = true
	return nil
}
```

创建 `app/server/internal/signal/nats_event_bus_test.go`：

```go
package signal

import (
	"encoding/json"
	"testing"
)

func TestNATSEventBus_PublishNamespace_DeliversLocalAndPublishesEnvelope(t *testing.T) {
	local := &recordingEventBus{}
	client := newFakeNATSClient()
	bus := NewNATSEventBus(client, local, "node-a", "gospeak")
	if err := bus.Start(); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	payload := map[string]interface{}{"room": "alpha"}
	if err := bus.PublishNamespace(EventRoomUpdated, payload); err != nil {
		t.Fatalf("unexpected publish error: %v", err)
	}

	if len(local.namespace) != 1 {
		t.Fatalf("expected 1 local namespace event, got %d", len(local.namespace))
	}
	if len(client.published) != 1 {
		t.Fatalf("expected 1 published NATS message, got %d", len(client.published))
	}

	var env EventEnvelope
	if err := json.Unmarshal(client.published[0].data, &env); err != nil {
		t.Fatalf("failed to decode envelope: %v", err)
	}
	if env.NodeID != "node-a" {
		t.Fatalf("expected node-a, got %q", env.NodeID)
	}
	if env.Scope != EventScopeNamespace {
		t.Fatalf("expected namespace scope, got %q", env.Scope)
	}
	if env.Event != EventRoomUpdated {
		t.Fatalf("expected event %q, got %q", EventRoomUpdated, env.Event)
	}
}

func TestNATSEventBus_RemoteRoomEvent_DeliversToLocalBus(t *testing.T) {
	local := &recordingEventBus{}
	client := newFakeNATSClient()
	bus := NewNATSEventBus(client, local, "node-a", "gospeak")
	if err := bus.Start(); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	rawPayload, err := json.Marshal(map[string]interface{}{"targetIdentity": "bob"})
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	rawEvent, err := json.Marshal(EventEnvelope{
		NodeID: "node-b",
		Scope:  EventScopeRoom,
		Room:   "alpha",
		Event:  EventRoomKicked,
		Data:   rawPayload,
	})
	if err != nil {
		t.Fatalf("failed to marshal envelope: %v", err)
	}

	client.Emit("gospeak.signal.room", rawEvent)

	if len(local.rooms) != 1 {
		t.Fatalf("expected 1 local room event, got %d", len(local.rooms))
	}
	if local.rooms[0].room != "alpha" {
		t.Fatalf("expected room alpha, got %q", local.rooms[0].room)
	}
	if local.rooms[0].event != EventRoomKicked {
		t.Fatalf("expected event %q, got %q", EventRoomKicked, local.rooms[0].event)
	}
	data, ok := local.rooms[0].data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map payload, got %T", local.rooms[0].data)
	}
	if data["targetIdentity"] != "bob" {
		t.Fatalf("expected bob, got %#v", data["targetIdentity"])
	}
}

func TestNATSEventBus_IgnoresOwnMessages(t *testing.T) {
	local := &recordingEventBus{}
	client := newFakeNATSClient()
	bus := NewNATSEventBus(client, local, "node-a", "gospeak")
	if err := bus.Start(); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	rawPayload, err := json.Marshal(map[string]interface{}{"room": "alpha"})
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	rawEvent, err := json.Marshal(EventEnvelope{
		NodeID: "node-a",
		Scope:  EventScopeNamespace,
		Event:  EventRoomUpdated,
		Data:   rawPayload,
	})
	if err != nil {
		t.Fatalf("failed to marshal envelope: %v", err)
	}

	client.Emit("gospeak.signal.namespace", rawEvent)

	if len(local.namespace) != 0 {
		t.Fatalf("expected no local replay for self messages, got %d", len(local.namespace))
	}
}
```

- [ ] **步骤 2：运行测试确认其失败**

运行：`cd app/server && go test ./internal/signal -run TestNATSEventBus -v`

预期：FAIL，报 `undefined: NewNATSEventBus`

- [ ] **步骤 3：编写最小化实现**

创建 `app/server/internal/signal/nats_event_bus.go`：

```go
package signal

import (
	"encoding/json"
	"fmt"
	"io"
)

const defaultNATSSubjectPrefix = "gospeak"

type natsClient interface {
	Publish(subject string, data []byte) error
	Subscribe(subject string, handler func(subject string, data []byte)) (io.Closer, error)
	Drain() error
	Close() error
}

type NATSEventBus struct {
	client        natsClient
	local         EventBus
	nodeID        string
	subjectPrefix string
	namespaceSub  io.Closer
	roomSub       io.Closer
}

func NewNATSEventBus(client natsClient, local EventBus, nodeID, subjectPrefix string) *NATSEventBus {
	if subjectPrefix == "" {
		subjectPrefix = defaultNATSSubjectPrefix
	}
	return &NATSEventBus{
		client:        client,
		local:         local,
		nodeID:        nodeID,
		subjectPrefix: subjectPrefix,
	}
}

func (b *NATSEventBus) Start() error {
	namespaceSub, err := b.client.Subscribe(b.namespaceSubject(), b.handleMessage)
	if err != nil {
		return err
	}
	roomSub, err := b.client.Subscribe(b.roomSubject(), b.handleMessage)
	if err != nil {
		_ = namespaceSub.Close()
		return err
	}
	b.namespaceSub = namespaceSub
	b.roomSub = roomSub
	return nil
}

func (b *NATSEventBus) PublishNamespace(event string, data interface{}) error {
	if b.local != nil {
		if err := b.local.PublishNamespace(event, data); err != nil {
			return err
		}
	}
	payload, err := b.marshalEnvelope(EventScopeNamespace, "", event, data)
	if err != nil {
		return err
	}
	return b.client.Publish(b.namespaceSubject(), payload)
}

func (b *NATSEventBus) PublishRoom(room, event string, data interface{}) error {
	if b.local != nil {
		if err := b.local.PublishRoom(room, event, data); err != nil {
			return err
		}
	}
	payload, err := b.marshalEnvelope(EventScopeRoom, room, event, data)
	if err != nil {
		return err
	}
	return b.client.Publish(b.roomSubject(), payload)
}

func (b *NATSEventBus) Close() error {
	if b.roomSub != nil {
		_ = b.roomSub.Close()
	}
	if b.namespaceSub != nil {
		_ = b.namespaceSub.Close()
	}
	if err := b.client.Drain(); err != nil {
		_ = b.client.Close()
		return err
	}
	return b.client.Close()
}

func (b *NATSEventBus) marshalEnvelope(scope, room, event string, data interface{}) ([]byte, error) {
	rawData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return json.Marshal(EventEnvelope{
		NodeID: b.nodeID,
		Scope:  scope,
		Room:   room,
		Event:  event,
		Data:   rawData,
	})
}

func (b *NATSEventBus) handleMessage(_ string, data []byte) {
	var envelope EventEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return
	}
	if envelope.NodeID == b.nodeID {
		return
	}
	var payload interface{}
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		return
	}
	switch envelope.Scope {
	case EventScopeNamespace:
		if b.local != nil {
			_ = b.local.PublishNamespace(envelope.Event, payload)
		}
	case EventScopeRoom:
		if b.local != nil {
			_ = b.local.PublishRoom(envelope.Room, envelope.Event, payload)
		}
	}
}

func (b *NATSEventBus) namespaceSubject() string {
	return fmt.Sprintf("%s.signal.namespace", b.subjectPrefix)
}

func (b *NATSEventBus) roomSubject() string {
	return fmt.Sprintf("%s.signal.room", b.subjectPrefix)
}
```

- [ ] **步骤 4：运行测试确认通过**

运行：`cd app/server && go test ./internal/signal -run TestNATSEventBus -v`

预期：PASS

- [ ] **步骤 5：提交**

```bash
git add app/server/internal/signal/event_bus_test_helpers_test.go \
        app/server/internal/signal/nats_event_bus.go \
        app/server/internal/signal/nats_event_bus_test.go
git commit -m "feat(signal): add optional nats-backed replicated event bus"
```

---

### 任务 3：让 `signal.Hub` 通过事件总线发布

**文件：**
- 修改：`app/server/internal/signal/hub.go`
- 测试：`app/server/internal/signal/hub_event_bus_test.go`

- [ ] **步骤 1：编写会失败的测试**

创建 `app/server/internal/signal/hub_event_bus_test.go`：

```go
package signal

import "testing"

func TestHub_OnRoomCreate_UsesConfiguredEventBus(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.SetEventBus(&recordingEventBus{})

	conn := newMockConn("socket-1")
	bus := hub.eventBus.(*recordingEventBus)

	hub.OnRoomCreate(conn, `{"room":"alpha"}`)

	if len(bus.namespace) != 1 {
		t.Fatalf("expected 1 namespace event, got %d", len(bus.namespace))
	}
	if bus.namespace[0].event != EventRoomUpdated {
		t.Fatalf("expected %q, got %q", EventRoomUpdated, bus.namespace[0].event)
	}
}

func TestHub_OnMemberMicState_UsesConfiguredEventBus(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	bus := &recordingEventBus{}
	hub.SetEventBus(bus)

	conn := newMockConn("socket-1")
	if _, err := hub.OnRoomJoin(conn, `{"room":"alpha","identity":"alice"}`); err != nil {
		t.Fatalf("unexpected join error: %v", err)
	}
	if _, err := hub.OnRoomJoinSFU(conn, `{"room":"alpha","identity":"alice"}`); err != nil {
		t.Fatalf("unexpected sfu join error: %v", err)
	}

	hub.OnMemberMicState(conn, `{"room":"alpha","identity":"alice","isMicMuted":true}`)

	if len(bus.rooms) != 1 {
		t.Fatalf("expected 1 room event, got %d", len(bus.rooms))
	}
	if bus.rooms[0].room != "alpha" {
		t.Fatalf("expected room alpha, got %q", bus.rooms[0].room)
	}
	if bus.rooms[0].event != EventMemberUpdated {
		t.Fatalf("expected %q, got %q", EventMemberUpdated, bus.rooms[0].event)
	}
}
```

- [ ] **步骤 2：运行测试确认其失败**

运行：`cd app/server && go test ./internal/signal -run 'TestHub_.*ConfiguredEventBus' -v`

预期：FAIL，报 `hub.SetEventBus undefined` 或缺少事件总线行为

- [ ] **步骤 3：添加新的 Hub 字段、setter 和辅助方法**

在 `app/server/internal/signal/hub.go` 中，向 `Hub` 添加 `eventBus` 字段：

```go
type Hub struct {
	server              socketServer
	eventBus            EventBus
	sfuProvider         sfu.Provider
	streamResolver      StreamNameResolver
	rooms               map[string]*Room
	mu                  sync.RWMutex
	roomStore           roomStore
	muteStore           muteStore
	userStore           userStore
	permChecker         permChecker
	sfuSignalHandler    SFUSignalHandler
	activeStreams       map[string]struct{}
	roomStreams         map[string]map[string]struct{}
	streamRoomCache     map[string]string
	streamByIdentity    map[string]map[string]string
	participantCleanup  ParticipantCleanupHandler
}
```

在 `SetServer` 下方添加 setter：

```go
func (h *Hub) SetEventBus(bus EventBus) {
	h.eventBus = bus
}
```

在现有的 `BroadcastToRoom` 方法附近添加发布辅助方法：

```go
func (h *Hub) publishNamespace(event string, data interface{}) {
	if h.eventBus != nil {
		if err := h.eventBus.PublishNamespace(event, data); err == nil {
			return
		} else {
			log.Printf("[Signal] event bus namespace publish failed: event=%s err=%v", event, err)
		}
	}
	if h.server != nil {
		h.server.BroadcastToNamespace("/", event, data)
	}
}

func (h *Hub) publishRoom(room, event string, data interface{}) {
	if h.eventBus != nil {
		if err := h.eventBus.PublishRoom(room, event, data); err == nil {
			return
		} else {
			log.Printf("[Signal] event bus room publish failed: room=%s event=%s err=%v", room, event, err)
		}
	}
	if h.server != nil {
		h.server.BroadcastToRoom("/", room, event, data)
	}
}

func (h *Hub) BroadcastToRoom(room string, event string, data interface{}) {
	h.publishRoom(room, event, data)
}
```

更新 `BroadcastMute` 和 `BroadcastUnmute` 以使用辅助方法：

```go
func (h *Hub) BroadcastMute(userID uint, info *MuteInfo) {
	if h.server != nil || h.eventBus != nil {
		data := map[string]interface{}{
			"user_id":   userID,
			"permanent": info.Permanent,
			"reason":    info.Reason,
		}
		if !info.Permanent && info.ExpiresAt != "" {
			data["expires_at"] = info.ExpiresAt
		}
		h.publishNamespace(EventUserMuted, data)
	}
}

func (h *Hub) BroadcastUnmute(userID uint) {
	if h.server != nil || h.eventBus != nil {
		h.publishNamespace(EventUserUnmuted, map[string]interface{}{
			"user_id": userID,
		})
	}
}
```

- [ ] **步骤 4：将 Hub 中的直接广播调用替换为辅助方法**

在 `app/server/internal/signal/hub.go` 中，将 Hub 逻辑中每个直接的命名空间/房间广播替换为下面的辅助方法。

替换：

```go
h.server.BroadcastToNamespace("/", EventMemberLeft, map[string]interface{}{
	"room":     roomName,
	"identity": identity,
	"id":       s.ID(),
})
```

改为：

```go
h.publishNamespace(EventMemberLeft, map[string]interface{}{
	"room":     roomName,
	"identity": identity,
	"id":       s.ID(),
})
```

替换：

```go
h.server.BroadcastToNamespace("/", EventRoomList, h.GetRooms())
```

改为：

```go
h.publishNamespace(EventRoomList, h.GetRooms())
```

替换：

```go
h.server.BroadcastToNamespace("/", EventRoomUpdated, info)
```

改为：

```go
h.publishNamespace(EventRoomUpdated, info)
```

替换：

```go
h.server.BroadcastToRoom("/", req.Room, EventRoomKicked, map[string]interface{}{
	"room":           req.Room,
	"targetIdentity": req.TargetIdentity,
})
```

改为：

```go
h.publishRoom(req.Room, EventRoomKicked, map[string]interface{}{
	"room":           req.Room,
	"targetIdentity": req.TargetIdentity,
})
```

然后按以下方式替换其余的直接调用：

```go
// OnDisconnect
h.server.BroadcastToNamespace("/", EventRoomList, h.GetRooms())
// => h.publishNamespace(EventRoomList, h.GetRooms())

h.server.BroadcastToNamespace("/", EventRoomUpdated, info)
// => h.publishNamespace(EventRoomUpdated, info)

// OnRoomCreate
h.server.BroadcastToNamespace("/", EventRoomUpdated, roomInfo)
// => h.publishNamespace(EventRoomUpdated, roomInfo)

// OnRoomJoinSFU
h.server.BroadcastToNamespace("/", EventMemberJoined, map[string]interface{}{
	"room":     req.Room,
	"identity": req.Identity,
	"id":       s.ID(),
	"stream":   req.Stream,
})
// => h.publishNamespace(EventMemberJoined, map[string]interface{}{...})

h.server.BroadcastToNamespace("/", EventRoomUpdated, info)
// => h.publishNamespace(EventRoomUpdated, info)

// OnRoomLeave
h.server.BroadcastToNamespace("/", EventMemberLeft, map[string]interface{}{
	"room":     req.Room,
	"identity": identity,
	"id":       s.ID(),
})
// => h.publishNamespace(EventMemberLeft, map[string]interface{}{...})

h.server.BroadcastToNamespace("/", EventRoomList, h.GetRooms())
// => h.publishNamespace(EventRoomList, h.GetRooms())

h.server.BroadcastToNamespace("/", EventRoomUpdated, info)
// => h.publishNamespace(EventRoomUpdated, info)

// OnRoomKick
h.server.BroadcastToNamespace("/", EventMemberLeft, map[string]interface{}{
	"room":     req.Room,
	"identity": req.TargetIdentity,
	"id":       targetSocketID,
})
// => h.publishNamespace(EventMemberLeft, map[string]interface{}{...})

h.server.BroadcastToNamespace("/", EventRoomList, h.GetRooms())
// => h.publishNamespace(EventRoomList, h.GetRooms())

h.server.BroadcastToNamespace("/", EventRoomUpdated, info)
// => h.publishNamespace(EventRoomUpdated, info)
```

这些替换完成后，`hub.go` 中留给直接的广播调用的只有在 `publishNamespace` 和 `publishRoom` 内部的本地回退。

- [ ] **步骤 5：运行测试确认通过**

运行：`cd app/server && go test ./internal/signal -run 'TestHub_.*ConfiguredEventBus' -v`

预期：PASS

- [ ] **步骤 6：提交**

```bash
git add app/server/internal/signal/hub.go \
        app/server/internal/signal/hub_event_bus_test.go
git commit -m "refactor(signal): route hub broadcasts through event bus"
```

---

### 任务 4：添加配置驱动的事件总线构建和真实 NATS 客户端适配器

**文件：**
- 修改：`app/server/internal/config/config.go`
- 创建：`app/server/internal/signal/nats_client.go`
- 创建：`app/server/internal/signal/bus_factory.go`
- 测试：`app/server/internal/signal/bus_factory_test.go`
- 修改：`app/server/go.mod`
- 修改：`app/server/go.sum`

- [ ] **步骤 1：编写会失败的测试**

创建 `app/server/internal/signal/bus_factory_test.go`：

```go
package signal

import (
	"testing"

	"GOSpeak/internal/config"
)

func TestBuildEventBus_WithoutNATS_ReturnsLocalBus(t *testing.T) {
	server := newMockServer()

	bus, closeFn, err := BuildEventBus(&config.Config{}, server, "node-a", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := bus.PublishNamespace(EventRoomUpdated, map[string]interface{}{"room": "alpha"}); err != nil {
		t.Fatalf("unexpected publish error: %v", err)
	}
	if len(server.broadcasts[EventRoomUpdated]) != 1 {
		t.Fatalf("expected local broadcast, got %d payloads", len(server.broadcasts[EventRoomUpdated]))
	}
	if err := closeFn(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
}

func TestBuildEventBus_WithNATS_ReturnsReplicatedBus(t *testing.T) {
	server := newMockServer()
	client := newFakeNATSClient()
	dialed := false

	bus, closeFn, err := BuildEventBus(&config.Config{
		NATSURL:           "nats://127.0.0.1:4222",
		NATSSubjectPrefix: "gospeak",
	}, server, "node-a", func(url string) (natsClient, error) {
		dialed = true
		if url != "nats://127.0.0.1:4222" {
			t.Fatalf("unexpected url: %s", url)
		}
		return client, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dialed {
		t.Fatal("expected dialer to be called")
	}
	if err := bus.PublishNamespace(EventRoomUpdated, map[string]interface{}{"room": "alpha"}); err != nil {
		t.Fatalf("unexpected publish error: %v", err)
	}
	if len(server.broadcasts[EventRoomUpdated]) != 1 {
		t.Fatalf("expected local broadcast, got %d payloads", len(server.broadcasts[EventRoomUpdated]))
	}
	if len(client.published) != 1 {
		t.Fatalf("expected 1 replicated NATS message, got %d", len(client.published))
	}
	if err := closeFn(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
	if !client.drained {
		t.Fatal("expected client to drain on close")
	}
	if !client.closed {
		t.Fatal("expected client to close on close")
	}
}
```

- [ ] **步骤 2：运行测试确认其失败**

运行：`cd app/server && go test ./internal/signal -run TestBuildEventBus -v`

预期：FAIL，报 `undefined: BuildEventBus` 和缺失配置字段

- [ ] **步骤 3：实现配置字段、真实 NATS 客户端和工厂**

在 `app/server/internal/config/config.go` 中，向 `Config` 添加新字段：

```go
	NATSURL            string
	NATSSubjectPrefix  string
```

在 `Load()` 返回的结构体中添加：

```go
		NATSURL:            getEnv("NATS_URL", ""),
		NATSSubjectPrefix:  getEnv("NATS_SUBJECT_PREFIX", "gospeak"),
```

创建 `app/server/internal/signal/nats_client.go`：

```go
package signal

import (
	"io"

	"github.com/nats-io/nats.go"
)

type realNATSSubscription struct {
	sub *nats.Subscription
}

func (s *realNATSSubscription) Close() error {
	return s.sub.Unsubscribe()
}

type realNATSClient struct {
	conn *nats.Conn
}

func NewNATSClient(url string) (natsClient, error) {
	conn, err := nats.Connect(url, nats.Name("gospeak-signal"))
	if err != nil {
		return nil, err
	}
	return &realNATSClient{conn: conn}, nil
}

func (c *realNATSClient) Publish(subject string, data []byte) error {
	return c.conn.Publish(subject, data)
}

func (c *realNATSClient) Subscribe(subject string, handler func(subject string, data []byte)) (io.Closer, error) {
	sub, err := c.conn.Subscribe(subject, func(msg *nats.Msg) {
		handler(msg.Subject, msg.Data)
	})
	if err != nil {
		return nil, err
	}
	return &realNATSSubscription{sub: sub}, nil
}

func (c *realNATSClient) Drain() error {
	return c.conn.Drain()
}

func (c *realNATSClient) Close() error {
	c.conn.Close()
	return nil
}
```

创建 `app/server/internal/signal/bus_factory.go`：

```go
package signal

import "GOSpeak/internal/config"

type natsDialer func(url string) (natsClient, error)

func BuildEventBus(cfg *config.Config, server socketServer, nodeID string, dial natsDialer) (EventBus, func() error, error) {
	local := NewLocalEventBus(server)
	if cfg == nil || cfg.NATSURL == "" {
		return local, local.Close, nil
	}
	if dial == nil {
		dial = NewNATSClient
	}
	client, err := dial(cfg.NATSURL)
	if err != nil {
		return nil, nil, err
	}
	bus := NewNATSEventBus(client, local, nodeID, cfg.NATSSubjectPrefix)
	if err := bus.Start(); err != nil {
		_ = client.Close()
		return nil, nil, err
	}
	return bus, bus.Close, nil
}
```

- [ ] **步骤 4：将 NATS 依赖提升为直接模块依赖**

运行：`cd app/server && go mod tidy`

预期：`app/server/go.mod` 保持 `github.com/nats-io/nats.go` 为直接依赖，并按需更新 `go.sum`

- [ ] **步骤 5：运行测试确认通过**

运行：`cd app/server && go test ./internal/signal -run TestBuildEventBus -v`

预期：PASS

- [ ] **步骤 6：提交**

```bash
git add app/server/internal/config/config.go \
        app/server/internal/signal/nats_client.go \
        app/server/internal/signal/bus_factory.go \
        app/server/internal/signal/bus_factory_test.go \
        app/server/go.mod \
        app/server/go.sum
git commit -m "feat(signal): build local or nats event bus from config"
```

---

### 任务 5：将总线接入服务器启动并编写部署文档

**文件：**
- 修改：`app/server/server/gin.go`
- 修改：`deploy/docker-compose.example.yml`
- 修改：`ARCHITECTURE.md`

- [ ] **步骤 1：将事件总线接入服务器启动和关闭**

在 `app/server/server/gin.go` 中，添加 UUID 导入：

```go
	"github.com/google/uuid"
```

在 `sioServer := socketio.NewServer(...)` 之后，定义一个关闭函数的默认值：

```go
	closeEventBus := func() error { return nil }
```

然后将当前的 Hub 设置块替换为以下精确顺序：

```go
	signalHub := signal.NewHub(roomSvc, muteSvc, userSvc, permSvc)
	signalHub.SetSFU(sfuProvider)
	if snr, ok := sfuProvider.(signal.StreamNameResolver); ok {
		signalHub.SetStreamResolver(snr)
	}
	eventBus, closeBus, err := signal.BuildEventBus(cfg, sioServer, uuid.NewString(), nil)
	if err != nil {
		panic(fmt.Sprintf("failed to build signal event bus: %v", err))
	}
	closeEventBus = closeBus
	signalHub.SetEventBus(eventBus)
	if rs, ok := sfuProvider.(pkg.RoomRegistrySetter); ok {
		rs.SetRoomRegistry(signalHub)
	}
	if resolvedSFUCfg.SFUProvider == "mediasoup" {
		msService := mediasoup.NewService(resolvedSFUCfg)
		msSignal := mediasoup.NewMediasoupSignal(msService.Bridge, signalHub.BroadcastToRoom)
		signalHub.SetSFUSignalHandler(msSignal)
	}
	signalHub.SetupRoutes(sioServer)
```

在优雅关闭的 goroutine 中，在关闭 Socket.IO 后关闭事件总线：

```go
		if err := sioServer.Close(); err != nil {
			log.Printf("[Socket.IO] close error: %v", err)
		}
		log.Println("[Socket.IO] connections closed")

		if err := closeEventBus(); err != nil {
			log.Printf("[Signal] event bus close error: %v", err)
		}
```

- [ ] **步骤 2：为本地部署添加可选的 NATS 服务**

在 `deploy/docker-compose.example.yml` 中，直接在 `services:` 下方添加服务：

```yaml
  nats:
    image: nats:2-alpine
    container_name: gospeak-nats
    restart: unless-stopped
    ports:
      - "4222:4222"
      - "8222:8222"
```

- [ ] **步骤 3：编写新的可选运行时路径文档**

在 `ARCHITECTURE.md` 中，在后端运行时/部署部分附近添加此章节：

````markdown
## 可选的 NATS 信号事件总线

GOSpeak 现在支持一个可选的基于 NATS 的信号事件总线。

- 默认行为保持不变：如果 `NATS_URL` 为空，Socket.IO 事件仅在当前进程内广播。
- 如果设置了 `NATS_URL`，服务器仍然先将事件投递到自己的本地 Socket，然后将 JSON 信封重新发布到 NATS，以便对等 GOSpeak 实例可以将同一事件重放到它们自己的 Socket.IO 客户端。
- 此更改**尚未**分发 `Hub.rooms`、静音状态或 SRS 流状态。它仅提取了广播路径。

环境变量：

```env
NATS_URL=""                    # 空值禁用 NATS；单实例默认值
NATS_SUBJECT_PREFIX="gospeak"  # 信号主题的命名空间前缀
```

主题布局：

- `gospeak.signal.namespace`
- `gospeak.signal.room`
````

- [ ] **步骤 4：运行验证命令**

运行：

```bash
cd app/server && go test ./internal/signal -run 'Test(LocalEventBus|NATSEventBus|BuildEventBus|Hub_.*ConfiguredEventBus)' -v
cd app/server && go build ./...
cd deploy && docker compose -f docker-compose.example.yml config >/tmp/gospeak-compose.txt
```

预期：

- 目标信号测试全部 PASS
- `go build ./...` 成功
- `docker compose config` 退出码 0 且输出中包含 `nats` 服务

- [ ] **步骤 5：提交**

```bash
git add app/server/server/gin.go \
        deploy/docker-compose.example.yml \
        ARCHITECTURE.md
git commit -m "feat(signal): wire optional nats event bus into server runtime"
```

---

## 自我审查

### 1. 规范覆盖范围

本计划涵盖：

- 将信号事件扇出提取到稳定接口后面
- 保持当前单节点路径作为默认行为
- 添加可选的 NATS 复制（带同节点去重）
- 接入配置、启动、关闭和部署文档

设计上特意排除：

- 分布式房间成员状态
- 分布式 SRS 活动流注册表
- 跨实例的审核状态协调

这些是独立的子系统，应该有自己的计划。

### 2. 占位符检查

检查了：`TBD`、`TODO`、`implement later`、`appropriate error handling`、`similar to Task N`。

结果：无残留占位符。

### 3. 类型一致性

验证了跨任务的一致命名：

- `EventBus`
- `EventEnvelope`
- `LocalEventBus`
- `NATSEventBus`
- `BuildEventBus`
- `SetEventBus`
- `NATSURL`
- `NATSSubjectPrefix`

---
