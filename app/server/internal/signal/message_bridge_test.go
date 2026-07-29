package signal

import (
	"context"
	"testing"

	"GOSpeak/internal/bus"
	"GOSpeak/internal/model"
	"GOSpeak/internal/service"
)

// stubMsgSvc records the last Send call for test assertions.
type stubMsgSvc struct {
	last service.MessageSendInput
	n    int
}

func (s *stubMsgSvc) Send(ctx context.Context, in service.MessageSendInput) (*service.MessageDTO, error) {
	s.n++
	s.last = in
	return &service.MessageDTO{ID: "01TEST", Room: in.RoomKey, Content: in.Content}, nil
}

func TestOnMessageSend_SendsToMember(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "room-a", map[string]string{
		"sock-1": "alice",
	})

	svc := &stubMsgSvc{}
	hub.SetMessageService(svc)

	conn := newAuthedMockClient("sock-1", "alice")
	hub.OnMessageSend(conn, `{"room":"room-a","content":"hello world"}`)

	if svc.n != 1 {
		t.Fatalf("expected 1 Send call, got %d", svc.n)
	}
	if svc.last.Content != "hello world" {
		t.Errorf("expected content 'hello world', got %q", svc.last.Content)
	}
	if svc.last.RoomKey != "room-a" {
		t.Errorf("expected RoomKey 'room-a', got %q", svc.last.RoomKey)
	}
	if svc.last.SenderIdentity != "alice" {
		t.Errorf("expected SenderIdentity 'alice', got %q", svc.last.SenderIdentity)
	}
}

func TestOnMessageSend_NotInRoom_Denied(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "room-a", map[string]string{
		"sock-1": "alice",
	})

	svc := &stubMsgSvc{}
	hub.SetMessageService(svc)

	conn := newAuthedMockClient("sock-2", "bob")
	hub.OnMessageSend(conn, `{"room":"room-a","content":"hi"}`)

	if svc.n != 0 {
		t.Fatalf("expected 0 Send calls for non-member, got %d", svc.n)
	}
}

func TestOnMessageSend_EmptyContent_Denied(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "room-a", map[string]string{
		"sock-1": "alice",
	})

	svc := &stubMsgSvc{}
	hub.SetMessageService(svc)

	conn := newAuthedMockClient("sock-1", "alice")
	hub.OnMessageSend(conn, `{"room":"room-a","content":""}`)

	if svc.n != 0 {
		t.Fatalf("expected 0 Send calls for empty content, got %d", svc.n)
	}
}

func TestOnMessageSend_Unauthenticated_Denied(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "room-a", map[string]string{
		"sock-1": "alice",
	})

	svc := &stubMsgSvc{}
	hub.SetMessageService(svc)

	conn := newMockClient("sock-1")
	hub.OnMessageSend(conn, `{"room":"room-a","content":"hi"}`)

	if svc.n != 0 {
		t.Fatalf("expected 0 Send calls for unauthenticated, got %d", svc.n)
	}
}

func TestOnMessageSend_FallbackTextToContent(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "room-a", map[string]string{
		"sock-1": "alice",
	})

	svc := &stubMsgSvc{}
	hub.SetMessageService(svc)

	conn := newAuthedMockClient("sock-1", "alice")
	hub.OnMessageSend(conn, `{"room":"room-a","text":"fallback text"}`)

	if svc.n != 1 {
		t.Fatalf("expected 1 Send call, got %d", svc.n)
	}
	if svc.last.Content != "fallback text" {
		t.Errorf("expected content 'fallback text', got %q", svc.last.Content)
	}
}

func TestOnMessageSend_WithReplyTo(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "room-a", map[string]string{
		"sock-1": "alice",
	})

	svc := &stubMsgSvc{}
	hub.SetMessageService(svc)

	conn := newAuthedMockClient("sock-1", "alice")
	hub.OnMessageSend(conn, `{"room":"room-a","content":"replying","replyTo":"msg-123"}`)

	if svc.n != 1 {
		t.Fatalf("expected 1 Send call, got %d", svc.n)
	}
	if svc.last.ReplyToID != "msg-123" {
		t.Errorf("expected ReplyToID 'msg-123', got %q", svc.last.ReplyToID)
	}
}

func TestOnMessageSend_NilService_Noop(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "room-a", map[string]string{
		"sock-1": "alice",
	})

	conn := newAuthedMockClient("sock-1", "alice")
	// messageSvc is nil by default — should not panic
	hub.OnMessageSend(conn, `{"room":"room-a","content":"hi"}`)
}

func TestOnMessageSend_InvalidJSON_Denied(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "room-a", map[string]string{
		"sock-1": "alice",
	})

	svc := &stubMsgSvc{}
	hub.SetMessageService(svc)

	conn := newAuthedMockClient("sock-1", "alice")
	hub.OnMessageSend(conn, `not json`)

	if svc.n != 0 {
		t.Fatalf("expected 0 Send calls for invalid JSON, got %d", svc.n)
	}
}

func TestOnMessageSend_MissingRoom_Denied(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "room-a", map[string]string{
		"sock-1": "alice",
	})

	svc := &stubMsgSvc{}
	hub.SetMessageService(svc)

	conn := newAuthedMockClient("sock-1", "alice")
	hub.OnMessageSend(conn, `{"content":"hi"}`)

	if svc.n != 0 {
		t.Fatalf("expected 0 Send calls for missing room, got %d", svc.n)
	}
}

// stubMuteStore always returns muted.
type stubMuteStore struct{}
func (m *stubMuteStore) IsMutedByIdentity(identity string) (bool, *model.Mute, error) {
	return true, &model.Mute{UserID: 1, Permanent: true}, nil
}

func TestOnMessageSend_Muted_Denied(t *testing.T) {
	hub := NewHub(nil, &stubMuteStore{}, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "room-a", map[string]string{
		"sock-1": "alice",
	})
	svc := &stubMsgSvc{}
	hub.SetMessageService(svc)
	conn := newAuthedMockClient("sock-1", "alice")
	hub.OnMessageSend(conn, `{"room":"room-a","content":"hi"}`)
	if svc.n != 0 {
		t.Fatalf("expected 0 Send calls for muted user, got %d", svc.n)
	}
}

// ─── KV-priority member lookup tests ───

func TestOnMessageSend_KVPriority_FindsInKV(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()

	// KV store has alice, local rooms is empty
	kv := newMemStateStore()
	kv.PutRoomMembers(context.Background(), bus.RoomMembersSnapshot{
		Room: "room-a",
		Members: []bus.MemberRecord{
			{Room: "room-a", Identity: "alice", InstanceID: "inst-other"},
		},
	})
	hub.SetMembershipStore(kv, "inst-local")

	svc := &stubMsgSvc{}
	hub.SetMessageService(svc)

	conn := newAuthedMockClient("sock-1", "alice")
	hub.OnMessageSend(conn, `{"room":"room-a","content":"hi from kv"}`)

	if svc.n != 1 {
		t.Fatalf("expected 1 Send call, got %d", svc.n)
	}
	if svc.last.Content != "hi from kv" {
		t.Errorf("expected content 'hi from kv', got %q", svc.last.Content)
	}
	if svc.last.SenderIdentity != "alice" {
		t.Errorf("expected SenderIdentity 'alice', got %q", svc.last.SenderIdentity)
	}
}

func TestOnMessageSend_KVPriority_MissThenLocal(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	// alice is in local rooms
	seedKickRoom(hub, "room-a", map[string]string{"sock-1": "alice"})

	// KV store exists but room-a not in it yet
	kv := newMemStateStore()
	hub.SetMembershipStore(kv, "inst-local")

	svc := &stubMsgSvc{}
	hub.SetMessageService(svc)

	conn := newAuthedMockClient("sock-1", "alice")
	hub.OnMessageSend(conn, `{"room":"room-a","content":"fallback to local"}`)

	if svc.n != 1 {
		t.Fatalf("expected 1 Send call, got %d", svc.n)
	}
	if svc.last.Content != "fallback to local" {
		t.Errorf("expected content 'fallback to local', got %q", svc.last.Content)
	}
}

func TestOnMessageSend_KVPriority_NotFoundInBoth(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "room-a", map[string]string{"sock-1": "alice"})

	kv := newMemStateStore()
	hub.SetMembershipStore(kv, "inst-local")

	svc := &stubMsgSvc{}
	hub.SetMessageService(svc)

	conn := newAuthedMockClient("sock-2", "bob")
	hub.OnMessageSend(conn, `{"room":"room-a","content":"hi"}`)

	if svc.n != 0 {
		t.Fatalf("expected 0 Send calls for non-member, got %d", svc.n)
	}
}

func TestOnMessageSend_KVPriority_KVNotFound_FallsBackToLocal(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "room-a", map[string]string{"sock-1": "alice"})

	// KV returns error (room missing) → should fall back to local
	kv := newMemStateStore()
	hub.SetMembershipStore(kv, "inst-local")

	svc := &stubMsgSvc{}
	hub.SetMessageService(svc)

	conn := newAuthedMockClient("sock-1", "alice")
	hub.OnMessageSend(conn, `{"room":"room-a","content":"kv miss ok"}`)

	if svc.n != 1 {
		t.Fatalf("expected 1 Send call after KV miss -> local fallback, got %d", svc.n)
	}
}

func TestOnMessageSend_KVPriority_NoKV_LocalOnly(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "room-a", map[string]string{"sock-1": "alice"})

	// membershipStore is nil (no KV configured)
	svc := &stubMsgSvc{}
	hub.SetMessageService(svc)

	conn := newAuthedMockClient("sock-1", "alice")
	hub.OnMessageSend(conn, `{"room":"room-a","content":"no kv ok"}`)

	if svc.n != 1 {
		t.Fatalf("expected 1 Send call without KV, got %d", svc.n)
	}
}
