package signal

import (
	"context"
	"sync"
	"testing"

	"GOSpeak/internal/model"
)

// captureMuteBus 记录 namespace 事件与 payload，用于验证 mute 事件广播。
type captureMuteBus struct {
	mu sync.Mutex
	ns map[string][]map[string]interface{}
}

func (c *captureMuteBus) PublishNamespace(ctx context.Context, event string, payload interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ns == nil {
		c.ns = map[string][]map[string]interface{}{}
	}
	if m, ok := payload.(map[string]interface{}); ok {
		c.ns[event] = append(c.ns[event], m)
	} else {
		c.ns[event] = append(c.ns[event], nil)
	}
	return nil
}
func (c *captureMuteBus) PublishRoom(ctx context.Context, room, event string, payload interface{}) error {
	return nil
}
func (c *captureMuteBus) Mode() string       { return "test" }
func (c *captureMuteBus) IsConnected() bool  { return true }
func (c *captureMuteBus) InstanceID() string { return "test" }
func (c *captureMuteBus) Close() error       { return nil }

func TestBroadcastMute_PublishesMemberMuted(t *testing.T) {
	hub := NewHub(nil, nil, &idUserStore{users: map[uint]*model.User{
		7: {ID: 7, Name: "alice"},
	}}, nil)
	bus := &captureMuteBus{}
	hub.SetEventBus(bus)

	hub.BroadcastMute(7, &MuteInfo{Permanent: true})

	bus.mu.Lock()
	defer bus.mu.Unlock()
	events := bus.ns[EventMemberMuted]
	if len(events) != 1 {
		t.Fatalf("member:muted events = %d, want 1", len(events))
	}
	if events[0]["identity"] != "alice" || events[0]["muted"] != true {
		t.Fatalf("member:muted payload = %+v, want identity=alice muted=true", events[0])
	}
}

func TestBroadcastUnmute_PublishesMemberUnmuted(t *testing.T) {
	hub := NewHub(nil, nil, &idUserStore{users: map[uint]*model.User{
		7: {ID: 7, Name: "alice"},
	}}, nil)
	bus := &captureMuteBus{}
	hub.SetEventBus(bus)

	hub.BroadcastUnmute(7)

	bus.mu.Lock()
	defer bus.mu.Unlock()
	events := bus.ns[EventMemberUnmuted]
	if len(events) != 1 {
		t.Fatalf("member:unmuted events = %d, want 1", len(events))
	}
	if events[0]["identity"] != "alice" || events[0]["muted"] != false {
		t.Fatalf("member:unmuted payload = %+v, want identity=alice muted=false", events[0])
	}
}
