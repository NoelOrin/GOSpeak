package bus

import (
	"context"
	"sync"
	"testing"
	"time"
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
		URL:        url,
		SubjectPrefix: "gospeak",
		Name:       "inst-a",
		InstanceID: "inst-a",
		Mode:       "embedded",
		Deliverer:  d1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer b1.Close()

	b2, err := NewNATSBus(NATSBusConfig{
		URL:        url,
		SubjectPrefix: "gospeak",
		Name:       "inst-b",
		InstanceID: "inst-b",
		Mode:       "embedded",
		Deliverer:  d2,
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
