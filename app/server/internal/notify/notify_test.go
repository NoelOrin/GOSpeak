package notify

import (
	"context"
	"encoding/json"
	"testing"
)

type recordedPublish struct {
	room  string
	event string
	data  interface{}
}

type fakeBus struct {
	namespace []recordedPublish
	room      []recordedPublish
	err       error
}

func (b *fakeBus) PublishNamespace(_ context.Context, event string, payload interface{}) error {
	if b.err != nil {
		return b.err
	}
	b.namespace = append(b.namespace, recordedPublish{event: event, data: payload})
	return nil
}

func (b *fakeBus) PublishRoom(_ context.Context, room, event string, payload interface{}) error {
	if b.err != nil {
		return b.err
	}
	b.room = append(b.room, recordedPublish{room: room, event: event, data: payload})
	return nil
}

type samplePayload struct {
	UUID  string `json:"uuid"`
	Title string `json:"title"`
}

func TestNotifyUser_TargetsPersonalRoom(t *testing.T) {
	bus := &fakeBus{}
	payload := samplePayload{UUID: "a-1", Title: "维护公告"}

	if err := NotifyUser(context.Background(), bus, "user-uuid-1", "announcement:new", payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bus.room) != 1 {
		t.Fatalf("expected 1 room publish, got %d", len(bus.room))
	}
	if bus.room[0].room != "__user:user-uuid-1" {
		t.Errorf("room = %q, want %q", bus.room[0].room, "__user:user-uuid-1")
	}
	if bus.room[0].event != "announcement:new" {
		t.Errorf("event = %q", bus.room[0].event)
	}
	if len(bus.namespace) != 0 {
		t.Errorf("NotifyUser must not broadcast to namespace")
	}
}

func TestNotifyUser_PayloadStaysObject(t *testing.T) {
	bus := &fakeBus{}
	if err := NotifyUser(context.Background(), bus, "u1", "evt", samplePayload{UUID: "x", Title: "t"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, isBytes := bus.room[0].data.([]byte); isBytes {
		t.Fatal("payload must not be []byte")
	}
	raw, err := json.Marshal(map[string]interface{}{"event": bus.room[0].event, "data": bus.room[0].data})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	var decoded struct {
		Data struct {
			UUID  string `json:"uuid"`
			Title string `json:"title"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if decoded.Data.Title != "t" {
		t.Fatalf("payload was not encoded as an object; got %s", raw)
	}
}

func TestNotifyUser_RejectsEmptyUser(t *testing.T) {
	bus := &fakeBus{}
	if err := NotifyUser(context.Background(), bus, "", "evt", samplePayload{}); err == nil {
		t.Fatal("expected error for empty user uuid")
	}
	if len(bus.room) != 0 {
		t.Fatal("must not publish for empty user uuid")
	}
}

func TestNotifyUser_RejectsBytePayload(t *testing.T) {
	bus := &fakeBus{}
	if err := NotifyUser(context.Background(), bus, "u1", "evt", []byte(`{"a":1}`)); err == nil {
		t.Fatal("expected error for []byte payload")
	}
	if len(bus.room) != 0 {
		t.Fatal("must not publish []byte payload")
	}
}

func TestNotifyAll_UsesNamespace(t *testing.T) {
	bus := &fakeBus{}
	if err := NotifyAll(context.Background(), bus, "announcement:new", samplePayload{UUID: "g"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bus.namespace) != 1 {
		t.Fatalf("expected 1 namespace publish, got %d", len(bus.namespace))
	}
	if len(bus.room) != 0 {
		t.Fatal("NotifyAll must not publish to a room")
	}
}

func TestNotify_NilBusIsSafe(t *testing.T) {
	if err := NotifyUser(context.Background(), nil, "u1", "evt", samplePayload{}); err == nil {
		t.Fatal("expected error for nil bus")
	}
	if err := NotifyAll(context.Background(), nil, "evt", samplePayload{}); err == nil {
		t.Fatal("expected error for nil bus")
	}
}
