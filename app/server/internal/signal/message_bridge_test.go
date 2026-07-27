package signal

import (
	"context"
	"testing"

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
	hub.server = newMockServer()
	seedKickRoom(hub, "room-a", map[string]string{
		"sock-1": "alice",
	})

	svc := &stubMsgSvc{}
	hub.SetMessageService(svc)

	conn := newAuthedMockConn("sock-1", "alice")
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
	hub.server = newMockServer()
	seedKickRoom(hub, "room-a", map[string]string{
		"sock-1": "alice",
	})

	svc := &stubMsgSvc{}
	hub.SetMessageService(svc)

	conn := newAuthedMockConn("sock-2", "bob")
	hub.OnMessageSend(conn, `{"room":"room-a","content":"hi"}`)

	if svc.n != 0 {
		t.Fatalf("expected 0 Send calls for non-member, got %d", svc.n)
	}
}

func TestOnMessageSend_EmptyContent_Denied(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.server = newMockServer()
	seedKickRoom(hub, "room-a", map[string]string{
		"sock-1": "alice",
	})

	svc := &stubMsgSvc{}
	hub.SetMessageService(svc)

	conn := newAuthedMockConn("sock-1", "alice")
	hub.OnMessageSend(conn, `{"room":"room-a","content":""}`)

	if svc.n != 0 {
		t.Fatalf("expected 0 Send calls for empty content, got %d", svc.n)
	}
}

func TestOnMessageSend_Unauthenticated_Denied(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.server = newMockServer()
	seedKickRoom(hub, "room-a", map[string]string{
		"sock-1": "alice",
	})

	svc := &stubMsgSvc{}
	hub.SetMessageService(svc)

	conn := newMockConn("sock-1")
	hub.OnMessageSend(conn, `{"room":"room-a","content":"hi"}`)

	if svc.n != 0 {
		t.Fatalf("expected 0 Send calls for unauthenticated, got %d", svc.n)
	}
}

func TestOnMessageSend_FallbackTextToContent(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.server = newMockServer()
	seedKickRoom(hub, "room-a", map[string]string{
		"sock-1": "alice",
	})

	svc := &stubMsgSvc{}
	hub.SetMessageService(svc)

	conn := newAuthedMockConn("sock-1", "alice")
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
	hub.server = newMockServer()
	seedKickRoom(hub, "room-a", map[string]string{
		"sock-1": "alice",
	})

	svc := &stubMsgSvc{}
	hub.SetMessageService(svc)

	conn := newAuthedMockConn("sock-1", "alice")
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
	hub.server = newMockServer()
	seedKickRoom(hub, "room-a", map[string]string{
		"sock-1": "alice",
	})

	conn := newAuthedMockConn("sock-1", "alice")
	// messageSvc is nil by default — should not panic
	hub.OnMessageSend(conn, `{"room":"room-a","content":"hi"}`)
}

func TestOnMessageSend_InvalidJSON_Denied(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.server = newMockServer()
	seedKickRoom(hub, "room-a", map[string]string{
		"sock-1": "alice",
	})

	svc := &stubMsgSvc{}
	hub.SetMessageService(svc)

	conn := newAuthedMockConn("sock-1", "alice")
	hub.OnMessageSend(conn, `not json`)

	if svc.n != 0 {
		t.Fatalf("expected 0 Send calls for invalid JSON, got %d", svc.n)
	}
}

func TestOnMessageSend_MissingRoom_Denied(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.server = newMockServer()
	seedKickRoom(hub, "room-a", map[string]string{
		"sock-1": "alice",
	})

	svc := &stubMsgSvc{}
	hub.SetMessageService(svc)

	conn := newAuthedMockConn("sock-1", "alice")
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
	hub.server = newMockServer()
	seedKickRoom(hub, "room-a", map[string]string{
		"sock-1": "alice",
	})
	svc := &stubMsgSvc{}
	hub.SetMessageService(svc)
	conn := newAuthedMockConn("sock-1", "alice")
	hub.OnMessageSend(conn, `{"room":"room-a","content":"hi"}`)
	if svc.n != 0 {
		t.Fatalf("expected 0 Send calls for muted user, got %d", svc.n)
	}
}
