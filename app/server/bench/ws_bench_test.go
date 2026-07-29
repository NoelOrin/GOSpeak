package bench

import (
	"testing"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/ws"
)

type mockClient struct {
	id string
}

func (m *mockClient) ID() string                                              { return m.id }
func (m *mockClient) Claims() *pkg.Claims                                      { return nil }
func (m *mockClient) Send(v interface{}) bool                                  { return true }
func (m *mockClient) SendACK(id, event string, data interface{})               {}
func (m *mockClient) SendErrorACK(id, event string, code int, message string)  {}
func (m *mockClient) Close()                                                   {}

func BenchmarkBroadcastToRoom(b *testing.B) {
	fanout := ws.NewFanout()
	clients := make([]*ws.Client, 100)

	for i := 0; i < 100; i++ {
		id := string(rune('a' + i))
		clients[i] = ws.NewTestClient(id, nil)
		fanout.Add(clients[i])
		fanout.Join("testroom", id)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fanout.BroadcastToRoom("testroom", "event:bench", map[string]string{"data": "payload"})
	}
}

func BenchmarkFanoutJoinLeave(b *testing.B) {
	fanout := ws.NewFanout()
	clients := make([]*ws.Client, 50)

	for i := 0; i < 50; i++ {
		id := string(rune('a' + i))
		clients[i] = ws.NewTestClient(id, nil)
		fanout.Add(clients[i])
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		room := "room-" + string(rune('A'+(i%26)))
		for _, c := range clients {
			fanout.Join(room, c.ID())
		}
		for _, c := range clients {
			fanout.Leave(room, c.ID())
		}
	}
}

func BenchmarkMessageDispatch(b *testing.B) {
	reg := ws.NewHandlerRegistry()
	reg.Handle("event:bench", func(c ws.ClientMessenger, data string) {})

	msg := ws.Message{
		ID:    "bench-1",
		Event: "event:bench",
		Data:  []byte(`{"key":"value","nested":{"a":1}}`),
	}

	tc := &mockClient{id: "bench-client"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reg.Dispatch(tc, msg)
	}
}
