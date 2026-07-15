package bus

import (
	"context"
	"sync"
	"testing"
	"time"
)

type recordedCall struct {
	event string
	data  interface{}
	room  string // empty for namespace calls
}

type recordingDeliverer struct {
	mu    sync.Mutex
	calls []recordedCall
}

func (d *recordingDeliverer) BroadcastToNamespace(event string, data interface{}) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, recordedCall{event: event, data: data})
}

func (d *recordingDeliverer) BroadcastToRoom(room, event string, data interface{}) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, recordedCall{event: event, data: data, room: room})
}

func TestNATSBus_PublishNamespace(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	defer es.Shutdown()

	d := &recordingDeliverer{}
	bus, err := NewNATSBus(NATSBusConfig{
		InstanceID:    "test",
		SubjectPrefix: "gospeak",
		URL:           es.ClientURL(),
		Deliverer:     d,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()

	time.Sleep(200 * time.Millisecond)

	payload := map[string]interface{}{"user": "alice"}
	if err := bus.PublishNamespace(context.Background(), "member:joined", payload); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	d.mu.Lock()
	calls := make([]recordedCall, len(d.calls))
	copy(calls, d.calls)
	d.mu.Unlock()

	if len(calls) != 1 {
		t.Fatalf("expected 1 local delivery, got %d", len(calls))
	}
	if calls[0].event != "member:joined" {
		t.Fatalf("expected event 'member:joined', got %q", calls[0].event)
	}
	if calls[0].room != "" {
		t.Fatalf("expected empty room for namespace, got %q", calls[0].room)
	}
}

func TestNATSBus_FanoutToPeerSkipsSelf(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	defer es.Shutdown()

	dA := &recordingDeliverer{}
	busA, err := NewNATSBus(NATSBusConfig{
		InstanceID:    "instance-a",
		SubjectPrefix: "gospeak",
		URL:           es.ClientURL(),
		Deliverer:     dA,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer busA.Close()

	dB := &recordingDeliverer{}
	busB, err := NewNATSBus(NATSBusConfig{
		InstanceID:    "instance-b",
		SubjectPrefix: "gospeak",
		URL:           es.ClientURL(),
		Deliverer:     dB,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer busB.Close()

	time.Sleep(300 * time.Millisecond)

	payload := map[string]interface{}{"user": "alice"}
	if err := busA.PublishNamespace(context.Background(), "member:joined", payload); err != nil {
		t.Fatal(err)
	}

	time.Sleep(500 * time.Millisecond)

	dA.mu.Lock()
	callsA := make([]recordedCall, len(dA.calls))
	copy(callsA, dA.calls)
	dA.mu.Unlock()

	if len(callsA) != 1 {
		t.Fatalf("busA: expected 1 local delivery, got %d (self NATS messages must be skipped)", len(callsA))
	}

	dB.mu.Lock()
	callsB := make([]recordedCall, len(dB.calls))
	copy(callsB, dB.calls)
	dB.mu.Unlock()

	if len(callsB) != 1 {
		t.Fatalf("busB: expected 1 event from peer, got %d", len(callsB))
	}
	if callsB[0].event != "member:joined" {
		t.Fatalf("busB: expected event 'member:joined', got %q", callsB[0].event)
	}
}
