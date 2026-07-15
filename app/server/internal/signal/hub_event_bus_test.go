package signal

import (
	"context"
	"sync"
	"testing"
)

type captureBus struct {
	mu    sync.Mutex
	rooms []string
	ns    []string
}

func (c *captureBus) PublishNamespace(ctx context.Context, event string, payload interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ns = append(c.ns, event)
	return nil
}
func (c *captureBus) PublishRoom(ctx context.Context, room, event string, payload interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rooms = append(c.rooms, room+":"+event)
	return nil
}
func (c *captureBus) Mode() string       { return "test" }
func (c *captureBus) IsConnected() bool  { return true }
func (c *captureBus) InstanceID() string { return "test" }
func (c *captureBus) Close() error       { return nil }

func TestHub_BroadcastToRoom_UsesEventBus(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	bus := &captureBus{}
	hub.SetEventBus(bus)
	// 即使 server 为 nil，也应走 bus
	hub.BroadcastToRoom("lobby", EventMemberJoined, map[string]string{"id": "1"})
	bus.mu.Lock()
	defer bus.mu.Unlock()
	if len(bus.rooms) != 1 || bus.rooms[0] != "lobby:"+EventMemberJoined {
		t.Fatalf("bus rooms = %v", bus.rooms)
	}
}

func TestHub_BroadcastMute_UsesEventBus(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	bus := &captureBus{}
	hub.SetEventBus(bus)
	hub.BroadcastMute(9, &MuteInfo{Permanent: true, Reason: "x"})
	bus.mu.Lock()
	defer bus.mu.Unlock()
	if len(bus.ns) != 1 || bus.ns[0] != EventUserMuted {
		t.Fatalf("bus ns = %v", bus.ns)
	}
}
