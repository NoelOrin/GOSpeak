# Optional NATS Signal Event Bus Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce an optional NATS-backed signal event bus for GOSpeak so Socket.IO room events can stay single-node by default and replicate cleanly across instances when `NATS_URL` is configured.

**Architecture:** Keep room membership, mute state, and SRS stream presence in-process for now; only extract the event fan-out path behind a new `signal.EventBus` interface. The default implementation remains a thin local adapter over `socketio.Server`, while the optional NATS implementation delivers locally and republishes envelopes to other nodes with same-node de-duplication.

**Tech Stack:** Go 1.26, Gin, go-socket.io, NATS Go client (`github.com/nats-io/nats.go`), Docker Compose, Go `testing`.

---

## Scope Check

This plan intentionally covers **one subsystem only**: optional NATS-based **signal event fan-out**.

It does **not** attempt to distribute these independent concerns in the same change:

- room membership state (`Hub.rooms`)
- SRS stream presence registry (`activeStreams`, `roomStreams`, `streamByIdentity`)
- SFU provider state reconciliation

Those should be separate plans if needed. This plan still produces working, testable software on its own because a single instance keeps using the local event bus, and multi-instance deployments gain replicated `room:*`, `member:*`, and `user:*` broadcasts.

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `app/server/internal/signal/event_bus.go` | Create | Event bus interface + shared NATS event envelope |
| `app/server/internal/signal/local_event_bus.go` | Create | Local bus that wraps `socketServer` namespace/room broadcasts |
| `app/server/internal/signal/local_event_bus_test.go` | Create | Unit tests for local namespace/room publishing |
| `app/server/internal/signal/event_bus_test_helpers_test.go` | Create | Shared test doubles: `recordingEventBus`, `fakeNATSClient` |
| `app/server/internal/signal/nats_event_bus.go` | Create | Optional replicated bus: local delivery + NATS publish/subscribe |
| `app/server/internal/signal/nats_event_bus_test.go` | Create | Unit tests for envelope encoding, remote delivery, self-message ignore |
| `app/server/internal/signal/hub.go` | Modify | Replace direct `server.Broadcast*` calls with `EventBus` helper methods |
| `app/server/internal/signal/hub_event_bus_test.go` | Create | Hub tests proving configured event bus is used |
| `app/server/internal/config/config.go` | Modify | Add `NATSURL` and `NATSSubjectPrefix` env-backed settings |
| `app/server/internal/signal/nats_client.go` | Create | Real NATS client adapter used by the bus factory |
| `app/server/internal/signal/bus_factory.go` | Create | Build local or NATS-backed bus from config |
| `app/server/internal/signal/bus_factory_test.go` | Create | Tests for config-driven bus selection |
| `app/server/server/gin.go` | Modify | Wire event bus into Hub startup and graceful shutdown |
| `app/server/go.mod` | Modify | Promote `github.com/nats-io/nats.go` to a direct dependency |
| `app/server/go.sum` | Modify | Dependency checksum updates after `go mod tidy` |
| `deploy/docker-compose.example.yml` | Modify | Optional NATS service for local multi-instance testing |
| `ARCHITECTURE.md` | Modify | Document the new optional event bus and env variables |

---

### Task 1: Create the signal event bus contract and local implementation

**Files:**
- Create: `app/server/internal/signal/event_bus.go`
- Create: `app/server/internal/signal/local_event_bus.go`
- Test: `app/server/internal/signal/local_event_bus_test.go`

- [ ] **Step 1: Write the failing tests**

Create `app/server/internal/signal/local_event_bus_test.go`:

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
	data, ok := got[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map payload, got %T", got[0])
	}
	if data["identity"] != "alice" {
		t.Fatalf("expected identity alice, got %#v", data["identity"])
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd app/server && go test ./internal/signal -run TestLocalEventBus -v`

Expected: FAIL with `undefined: NewLocalEventBus`

- [ ] **Step 3: Write the minimal implementation**

Create `app/server/internal/signal/event_bus.go`:

```go
package signal

import "encoding/json"

type EventBus interface {
	PublishNamespace(event string, data interface{}) error
	PublishRoom(room, event string, data interface{}) error
	Close() error
}

type EventEnvelope struct {
	NodeID string          `json:"nodeId"`
	Scope  string          `json:"scope"`
	Room   string          `json:"room,omitempty"`
	Event  string          `json:"event"`
	Data   json.RawMessage `json:"data"`
}

const (
	EventScopeNamespace = "namespace"
	EventScopeRoom      = "room"
)
```

Create `app/server/internal/signal/local_event_bus.go`:

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

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd app/server && go test ./internal/signal -run TestLocalEventBus -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/signal/event_bus.go \
        app/server/internal/signal/local_event_bus.go \
        app/server/internal/signal/local_event_bus_test.go
git commit -m "feat(signal): add local event bus abstraction"
```

---

### Task 2: Add the NATS-backed replicated event bus

**Files:**
- Create: `app/server/internal/signal/event_bus_test_helpers_test.go`
- Create: `app/server/internal/signal/nats_event_bus.go`
- Test: `app/server/internal/signal/nats_event_bus_test.go`

- [ ] **Step 1: Write the failing tests**

Create `app/server/internal/signal/event_bus_test_helpers_test.go`:

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

Create `app/server/internal/signal/nats_event_bus_test.go`:

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

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd app/server && go test ./internal/signal -run TestNATSEventBus -v`

Expected: FAIL with `undefined: NewNATSEventBus`

- [ ] **Step 3: Write the minimal implementation**

Create `app/server/internal/signal/nats_event_bus.go`:

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

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd app/server && go test ./internal/signal -run TestNATSEventBus -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/signal/event_bus_test_helpers_test.go \
        app/server/internal/signal/nats_event_bus.go \
        app/server/internal/signal/nats_event_bus_test.go
git commit -m "feat(signal): add optional nats-backed replicated event bus"
```

---

### Task 3: Teach `signal.Hub` to publish through the event bus

**Files:**
- Modify: `app/server/internal/signal/hub.go`
- Test: `app/server/internal/signal/hub_event_bus_test.go`

- [ ] **Step 1: Write the failing tests**

Create `app/server/internal/signal/hub_event_bus_test.go`:

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

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd app/server && go test ./internal/signal -run 'TestHub_.*ConfiguredEventBus' -v`

Expected: FAIL with `hub.SetEventBus undefined` or missing event bus behavior

- [ ] **Step 3: Add the new Hub field, setter, and helper methods**

In `app/server/internal/signal/hub.go`, add the `eventBus` field to `Hub`:

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

Add the setter below `SetServer`:

```go
func (h *Hub) SetEventBus(bus EventBus) {
	h.eventBus = bus
}
```

Add the publish helpers near the existing `BroadcastToRoom` method:

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

Update `BroadcastMute` and `BroadcastUnmute` to use the helper:

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

- [ ] **Step 4: Replace direct Hub broadcasts with helper calls**

In `app/server/internal/signal/hub.go`, replace each direct namespace/room broadcast in Hub logic with the helper methods below.

Replace:

```go
h.server.BroadcastToNamespace("/", EventMemberLeft, map[string]interface{}{
	"room":     roomName,
	"identity": identity,
	"id":       s.ID(),
})
```

With:

```go
h.publishNamespace(EventMemberLeft, map[string]interface{}{
	"room":     roomName,
	"identity": identity,
	"id":       s.ID(),
})
```

Replace:

```go
h.server.BroadcastToNamespace("/", EventRoomList, h.GetRooms())
```

With:

```go
h.publishNamespace(EventRoomList, h.GetRooms())
```

Replace:

```go
h.server.BroadcastToNamespace("/", EventRoomUpdated, info)
```

With:

```go
h.publishNamespace(EventRoomUpdated, info)
```

Replace:

```go
h.server.BroadcastToRoom("/", req.Room, EventRoomKicked, map[string]interface{}{
	"room":           req.Room,
	"targetIdentity": req.TargetIdentity,
})
```

With:

```go
h.publishRoom(req.Room, EventRoomKicked, map[string]interface{}{
	"room":           req.Room,
	"targetIdentity": req.TargetIdentity,
})
```

Then replace the remaining direct calls exactly as follows:

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

After these replacements, the only direct broadcast calls left in `hub.go` should be the local fallback inside `publishNamespace` and `publishRoom`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd app/server && go test ./internal/signal -run 'TestHub_.*ConfiguredEventBus' -v`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add app/server/internal/signal/hub.go \
        app/server/internal/signal/hub_event_bus_test.go
git commit -m "refactor(signal): route hub broadcasts through event bus"
```

---

### Task 4: Add config-driven event bus construction and the real NATS client adapter

**Files:**
- Modify: `app/server/internal/config/config.go`
- Create: `app/server/internal/signal/nats_client.go`
- Create: `app/server/internal/signal/bus_factory.go`
- Test: `app/server/internal/signal/bus_factory_test.go`
- Modify: `app/server/go.mod`
- Modify: `app/server/go.sum`

- [ ] **Step 1: Write the failing tests**

Create `app/server/internal/signal/bus_factory_test.go`:

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

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd app/server && go test ./internal/signal -run TestBuildEventBus -v`

Expected: FAIL with `undefined: BuildEventBus` and missing config fields

- [ ] **Step 3: Implement config fields, the real NATS client, and the factory**

In `app/server/internal/config/config.go`, add the new fields to `Config`:

```go
	NATSURL            string
	NATSSubjectPrefix  string
```

In the `Load()` return struct, add:

```go
		NATSURL:            getEnv("NATS_URL", ""),
		NATSSubjectPrefix:  getEnv("NATS_SUBJECT_PREFIX", "gospeak"),
```

Create `app/server/internal/signal/nats_client.go`:

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

Create `app/server/internal/signal/bus_factory.go`:

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

- [ ] **Step 4: Promote the NATS dependency to a direct module dependency**

Run: `cd app/server && go mod tidy`

Expected: `app/server/go.mod` keeps `github.com/nats-io/nats.go` as a direct dependency and updates `go.sum` if needed

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd app/server && go test ./internal/signal -run TestBuildEventBus -v`

Expected: PASS

- [ ] **Step 6: Commit**

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

### Task 5: Wire the bus into server startup and document deployment

**Files:**
- Modify: `app/server/server/gin.go`
- Modify: `deploy/docker-compose.example.yml`
- Modify: `ARCHITECTURE.md`

- [ ] **Step 1: Wire the event bus into server startup and shutdown**

In `app/server/server/gin.go`, add the UUID import:

```go
	"github.com/google/uuid"
```

Right after `sioServer := socketio.NewServer(...)`, define a close function default:

```go
	closeEventBus := func() error { return nil }
```

Then replace the current Hub setup block with the following exact sequence:

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

In the graceful shutdown goroutine, close the event bus after closing Socket.IO:

```go
		if err := sioServer.Close(); err != nil {
			log.Printf("[Socket.IO] close error: %v", err)
		}
		log.Println("[Socket.IO] connections closed")

		if err := closeEventBus(); err != nil {
			log.Printf("[Signal] event bus close error: %v", err)
		}
```

- [ ] **Step 2: Add an optional NATS service to local deployment**

In `deploy/docker-compose.example.yml`, add the service directly below `services:`:

```yaml
  nats:
    image: nats:2-alpine
    container_name: gospeak-nats
    restart: unless-stopped
    ports:
      - "4222:4222"
      - "8222:8222"
```

- [ ] **Step 3: Document the new optional runtime path**

In `ARCHITECTURE.md`, add this section near the backend runtime/deployment section:

````markdown
## Optional NATS Signal Event Bus

GOSpeak now supports an optional NATS-backed signal event bus.

- Default behavior stays unchanged: if `NATS_URL` is empty, Socket.IO events are broadcast only inside the current process.
- If `NATS_URL` is set, the server still delivers events to its own local sockets first, then republishes a JSON envelope to NATS so peer GOSpeak instances can replay the same event to their own Socket.IO clients.
- This change does **not** distribute `Hub.rooms`, mute state, or SRS stream presence yet. It only extracts the broadcast path.

Environment variables:

```env
NATS_URL=""                    # empty disables NATS; single-instance default
NATS_SUBJECT_PREFIX="gospeak"  # namespace prefix for signal subjects
```

Subject layout:

- `gospeak.signal.namespace`
- `gospeak.signal.room`
````

- [ ] **Step 4: Run the verification commands**

Run:

```bash
cd app/server && go test ./internal/signal -run 'Test(LocalEventBus|NATSEventBus|BuildEventBus|Hub_.*ConfiguredEventBus)' -v
cd app/server && go build ./...
cd deploy && docker compose -f docker-compose.example.yml config >/tmp/gospeak-compose.txt
```

Expected:

- targeted signal tests PASS
- `go build ./...` succeeds
- `docker compose config` exits 0 and includes the `nats` service

- [ ] **Step 5: Commit**

```bash
git add app/server/server/gin.go \
        deploy/docker-compose.example.yml \
        ARCHITECTURE.md
git commit -m "feat(signal): wire optional nats event bus into server runtime"
```

---

## Self-Review

### 1. Spec coverage

This plan covers:

- extracting signal event fan-out behind a stable interface
- preserving the current single-node path as the default
- adding optional NATS replication with same-node de-duplication
- wiring configuration, startup, shutdown, and deployment docs

Intentionally excluded by design:

- distributed room membership state
- distributed SRS active stream registry
- cross-instance moderation state reconciliation

Those are separate subsystems and should get their own plans.

### 2. Placeholder scan

Checked for: `TBD`, `TODO`, `implement later`, `appropriate error handling`, `similar to Task N`.

Result: none remain.

### 3. Type consistency

Verified consistent names across tasks:

- `EventBus`
- `EventEnvelope`
- `LocalEventBus`
- `NATSEventBus`
- `BuildEventBus`
- `SetEventBus`
- `NATSURL`
- `NATSSubjectPrefix`

---
