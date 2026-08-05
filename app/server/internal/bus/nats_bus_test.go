package bus

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

type memDeliverer struct {
	mu         sync.Mutex
	namespace  []string
	roomEvents []string
	payloads   []interface{}
}

func (m *memDeliverer) BroadcastToNamespace(event string, data interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.namespace = append(m.namespace, event)
	m.payloads = append(m.payloads, data)
}

func (m *memDeliverer) BroadcastToRoom(room, event string, data interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.roomEvents = append(m.roomEvents, room+":"+event)
	m.payloads = append(m.payloads, data)
}

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

type fanoutStub struct {
	onRoom      func(room, event string, data interface{})
	onNamespace func(event string, data interface{})
}

func (f fanoutStub) BroadcastToRoom(room, event string, data interface{}) {
	if f.onRoom != nil {
		f.onRoom(room, event, data)
	}
}

func (f fanoutStub) BroadcastToNamespace(event string, data interface{}) {
	if f.onNamespace != nil {
		f.onNamespace(event, data)
	}
}

func natsTestURL(t *testing.T) string {
	t.Helper()
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(es.Shutdown)
	return es.ClientURL()
}

func TestNATSBus_FanoutToPeerSkipsSelf(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	defer es.Shutdown()
	url := es.ClientURL()

	d1 := &memDeliverer{}
	d2 := &memDeliverer{}

	b1, err := NewNATSBus(NATSBusConfig{
		URL:           url,
		SubjectPrefix: "gospeak",
		Name:          "inst-a",
		InstanceID:    "inst-a",
		Mode:          "embedded",
		Deliverer:     d1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer b1.Close()

	b2, err := NewNATSBus(NATSBusConfig{
		URL:           url,
		SubjectPrefix: "gospeak",
		Name:          "inst-b",
		InstanceID:    "inst-b",
		Mode:          "embedded",
		Deliverer:     d2,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Close()

	if err := b1.PublishRoom(context.Background(), "r1", "member:joined", map[string]string{"id": "x"}); err != nil {
		t.Fatal(err)
	}

	waitUntil(t, 2*time.Second, func() bool {
		d1.mu.Lock()
		n1 := len(d1.roomEvents)
		d1.mu.Unlock()
		d2.mu.Lock()
		n2 := len(d2.roomEvents)
		d2.mu.Unlock()
		return n1 >= 1 && n2 >= 1
	})

	d1.mu.Lock()
	defer d1.mu.Unlock()
	d2.mu.Lock()
	defer d2.mu.Unlock()

	if len(d1.roomEvents) != 1 {
		t.Fatalf("local deliver want 1, got %v", d1.roomEvents)
	}
	if len(d2.roomEvents) != 1 {
		t.Fatalf("peer deliver want 1, got %v", d2.roomEvents)
	}
	// peer payload must be decoded object, not raw JSON bytes
	pm, ok := d2.payloads[0].(map[string]interface{})
	if !ok {
		t.Fatalf("peer payload type = %T, want map", d2.payloads[0])
	}
	if pm["id"] != "x" {
		t.Fatalf("peer payload = %#v", pm)
	}
}

func TestNATSBus_PublishNamespace(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	defer es.Shutdown()

	d1 := &memDeliverer{}
	d2 := &memDeliverer{}
	b1, err := NewNATSBus(NATSBusConfig{URL: es.ClientURL(), SubjectPrefix: "gospeak", InstanceID: "a", Name: "a", Mode: "external", Deliverer: d1})
	if err != nil {
		t.Fatal(err)
	}
	defer b1.Close()
	b2, err := NewNATSBus(NATSBusConfig{URL: es.ClientURL(), SubjectPrefix: "gospeak", InstanceID: "b", Name: "b", Mode: "external", Deliverer: d2})
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Close()

	if err := b1.PublishNamespace(context.Background(), "user:muted", map[string]any{"user_id": float64(1)}); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, 2*time.Second, func() bool {
		d2.mu.Lock()
		n := len(d2.namespace)
		d2.mu.Unlock()
		return n >= 1
	})
	d1.mu.Lock()
	d2.mu.Lock()
	defer d1.mu.Unlock()
	defer d2.mu.Unlock()
	if len(d1.namespace) != 1 || d1.namespace[0] != "user:muted" {
		t.Fatalf("local namespace = %v", d1.namespace)
	}
	if len(d2.namespace) != 1 || d2.namespace[0] != "user:muted" {
		t.Fatalf("peer namespace = %v", d2.namespace)
	}
	pm, ok := d2.payloads[0].(map[string]interface{})
	if !ok {
		t.Fatalf("peer payload type = %T", d2.payloads[0])
	}
	if pm["user_id"] != float64(1) {
		t.Fatalf("peer payload = %#v", pm)
	}
}

func TestNATSBus_CloseIdempotent(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	defer es.Shutdown()
	b, err := NewNATSBus(NATSBusConfig{
		URL: es.ClientURL(), SubjectPrefix: "gospeak", InstanceID: "c", Name: "c", Mode: "embedded", Deliverer: &memDeliverer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestNATSBus_PublishInternal_NoSocketDeliver(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	defer es.Shutdown()

	d1 := &memDeliverer{}
	d2 := &memDeliverer{}
	hookCh := make(chan string, 2)

	b1, err := NewNATSBus(NATSBusConfig{
		URL: es.ClientURL(), SubjectPrefix: "gospeak_int", InstanceID: "a", Name: "a", Mode: "embedded", Deliverer: d1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer b1.Close()

	b2, err := NewNATSBus(NATSBusConfig{
		URL: es.ClientURL(), SubjectPrefix: "gospeak_int", InstanceID: "b", Name: "b", Mode: "embedded", Deliverer: d2,
		RemoteHook: func(event string, payload interface{}) { hookCh <- event },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Close()

	if err := b1.PublishInternal(context.Background(), "cache:permissions-invalidated", map[string]string{"role": "user"}); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-hookCh:
		if ev != "cache:permissions-invalidated" {
			t.Fatalf("hook event %s", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting remote hook")
	}

	d1.mu.Lock()
	n1 := len(d1.namespace) + len(d1.roomEvents)
	d1.mu.Unlock()
	d2.mu.Lock()
	n2 := len(d2.namespace) + len(d2.roomEvents)
	d2.mu.Unlock()
	if n1 != 0 || n2 != 0 {
		t.Fatalf("deliverer should stay empty, d1=%d d2=%d", n1, n2)
	}
}

func TestNATSBus_PublishDisconnectedReturnsErrorAndCountsDrop(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	defer es.Shutdown()
	b, err := NewNATSBus(NATSBusConfig{
		URL:           es.ClientURL(),
		SubjectPrefix: "gospeak_drop",
		InstanceID:    "drop-1",
		Name:          "drop-1",
		Mode:          "external",
		Deliverer:     &memDeliverer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}

	if err := b.PublishRoom(context.Background(), "r1", "private:new", map[string]string{}); err == nil {
		t.Fatal("publish while disconnected should return error")
	}
	if got := b.DroppedPublishCount(); got != 1 {
		t.Fatalf("drop count = %d, want 1", got)
	}
	if err := b.PublishNamespace(context.Background(), "user:muted", map[string]string{}); err == nil {
		t.Fatal("namespace publish while disconnected should return error")
	}
	if got := b.DroppedPublishCount(); got != 2 {
		t.Fatalf("drop count = %d, want 2", got)
	}
}

func TestNATSBus_PublishFallbackFromExternalStaysSilent(t *testing.T) {
	b := &NATSBus{fallbackFromExternal: true}
	if err := b.PublishRoom(context.Background(), "r1", "member:joined", map[string]string{}); err != nil {
		t.Fatalf("fallback publish should stay silent: %v", err)
	}
	if got := b.DroppedPublishCount(); got != 0 {
		t.Fatalf("fallback drop count = %d, want 0", got)
	}
}

func TestNATSBus_OnMessageEnqueues(t *testing.T) {
	b, err := NewNATSBus(NATSBusConfig{
		URL:           natsTestURL(t),
		SubjectPrefix: "test",
		InstanceID:    "test-a",
		Mode:          "embedded",
	})
	if err != nil {
		t.Fatalf("NewNATSBus: %v", err)
	}
	defer b.Close()

	entered := make(chan struct{})
	release := make(chan struct{})
	defer close(release)

	var (
		enteredOnce sync.Once
		gotRoom     string
		gotEvent    string
		gotData     interface{}
	)

	b.SetDeliverer(fanoutStub{onRoom: func(room, event string, data interface{}) {
		gotRoom = room
		gotEvent = event
		gotData = data
		enteredOnce.Do(func() { close(entered) })
		<-release
	}})

	// 同步触发 onMessage 路径，异步 worker 应消费
	env, _ := NewEnvelope("test-b", "room", "r1", "room:kick", map[string]interface{}{"x": 1})
	raw, _ := json.Marshal(env)
	returned := make(chan struct{})
	go func() {
		b.onMessage(&nats.Msg{Data: raw})
		close(returned)
	}()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("expected delivery")
	}

	select {
	case <-returned:
		// onMessage 已返回，投递由异步 worker 完成
	case <-time.After(2 * time.Second):
		t.Fatal("expected async delivery")
	}

	if gotRoom != "r1" {
		t.Fatalf("room = %q, want r1", gotRoom)
	}
	if gotEvent != "room:kick" {
		t.Fatalf("event = %q, want room:kick", gotEvent)
	}
	dm, ok := gotData.(map[string]interface{})
	if !ok {
		t.Fatalf("data type = %T, want map[string]interface{}", gotData)
	}
	if dm["x"] != float64(1) {
		t.Fatalf("data = %#v, want x=1", dm)
	}
}

func TestNATSBus_PublishHonorsContext(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	defer es.Shutdown()
	url := es.ClientURL()

	localCh := make(chan string, 1)
	b1, err := NewNATSBus(NATSBusConfig{
		URL:           url,
		SubjectPrefix: "test",
		InstanceID:    "test-a",
		Name:          "test-a",
		Mode:          "embedded",
		Deliverer: fanoutStub{onRoom: func(room, event string, data interface{}) {
			localCh <- room + ":" + event
		}},
	})
	if err != nil {
		t.Fatalf("NewNATSBus b1: %v", err)
	}
	defer b1.Close()

	d2 := &memDeliverer{}
	b2, err := NewNATSBus(NATSBusConfig{
		URL:           url,
		SubjectPrefix: "test",
		InstanceID:    "test-b",
		Name:          "test-b",
		Mode:          "embedded",
		Deliverer:     d2,
	})
	if err != nil {
		t.Fatalf("NewNATSBus b2: %v", err)
	}
	defer b2.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// 本地投递仍应执行，NATS publish 已取消时立即返回错误
	err = b1.PublishRoom(ctx, "r1", "room:kick", map[string]interface{}{"x": 1})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("publish error = %v, want context.Canceled", err)
	}
	if got := b1.DroppedPublishCount(); got != 1 {
		t.Fatalf("drop count = %d, want 1", got)
	}
	select {
	case got := <-localCh:
		if got != "r1:room:kick" {
			t.Fatalf("local delivery = %q, want r1:room:kick", got)
		}
	default:
		t.Fatal("local delivery should still run before canceled publish")
	}

	// 取消的发布不应进入 NATS；短暂等待后直接断言 peer 未收到消息。
	time.Sleep(150 * time.Millisecond)
	d2.mu.Lock()
	n := len(d2.roomEvents)
	d2.mu.Unlock()
	if n != 0 {
		t.Fatalf("peer deliver count = %d, want 0", n)
	}
}

func TestNATSBus_PendingOverflowCounts(t *testing.T) {
	b := &NATSBus{}
	for i := 0; i < maxPendingPublish+5; i++ {
		b.enqueuePending("gospeak.signal.room.r1", Envelope{
			InstanceID: "inst-a",
			Scope:      "room",
			Room:       "r1",
			Event:      "room:updated",
		})
	}
	if got := b.DroppedPublishCount(); got != 5 {
		t.Fatalf("drop count = %d, want 5", got)
	}
	if len(b.pending) != maxPendingPublish {
		t.Fatalf("expected pending capped at %d, got %d", maxPendingPublish, len(b.pending))
	}
}
