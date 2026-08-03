package signal

import (
	"encoding/json"
	"testing"

	"GOSpeak/internal/pkg"
)

// decodeBotBroadcast extracts the string-encoded broadcastBotMessage from mockBroadcaster args.
func decodeBotBroadcast(t *testing.T, ms *mockBroadcaster, room, event string) *broadcastBotMessage {
	t.Helper()
	casts, ok := ms.roomCasts[room]
	if !ok {
		return nil
	}
	vals, ok := casts[event]
	if !ok || len(vals) == 0 {
		return nil
	}
	// BroadcastToRoom stores string payload as first variadic arg
	raw, ok := vals[0].(string)
	if !ok {
		return nil
	}
	var b broadcastBotMessage
	if err := json.Unmarshal([]byte(raw), &b); err != nil {
		t.Fatalf("unmarshal broadcast: %v", err)
	}
	return &b
}

func TestPublishBotCommand_BroadcastsToRoom(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "test-room", map[string]string{
		"socket-1": "user-1",
	})

	conn := newAuthedMockClient("socket-1", "user-1")
	hub.PublishBotCommand(conn, `{"room":"test-room","text":"/kick alice"}`)

	b := decodeBotBroadcast(t, hub.fanout.(*mockBroadcaster), "test-room", EventBotCommand)
	if b == nil {
		t.Fatal("expected bot:command broadcast")
	}
	if b.Content != "/kick alice" {
		t.Errorf("expected content '/kick alice', got %q", b.Content)
	}
	if b.From.Identity != "user-1" {
		t.Errorf("expected from identity 'user-1', got %q", b.From.Identity)
	}
	if b.Room != "test-room" {
		t.Errorf("expected room 'test-room', got %q", b.Room)
	}
	if b.MessageID == "" {
		t.Error("expected non-empty messageId")
	}
}

func TestPublishBotCommand_NotInRoom_Denied(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "test-room", map[string]string{
		"socket-1": "user-1",
	})

	conn := newAuthedMockClient("socket-2", "user-2")
	hub.PublishBotCommand(conn, `{"room":"test-room","text":"/kick alice"}`)

	b := decodeBotBroadcast(t, hub.fanout.(*mockBroadcaster), "test-room", EventBotCommand)
	if b != nil {
		t.Fatal("expected no broadcast for non-member")
	}
}

func TestPublishBotCommand_TextTooLong_Denied(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "test-room", map[string]string{
		"socket-1": "user-1",
	})

	longText := make([]byte, 501)
	for i := range longText {
		longText[i] = 'a'
	}

	conn := newAuthedMockClient("socket-1", "user-1")
	hub.PublishBotCommand(conn, `{"room":"test-room","text":"`+string(longText)+`"}`)

	b := decodeBotBroadcast(t, hub.fanout.(*mockBroadcaster), "test-room", EventBotCommand)
	if b != nil {
		t.Fatal("expected no broadcast for long text")
	}
}

func TestPublishBotCommand_EmptyText_Denied(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "test-room", map[string]string{
		"socket-1": "user-1",
	})

	conn := newAuthedMockClient("socket-1", "user-1")
	hub.PublishBotCommand(conn, `{"room":"test-room","text":""}`)

	b := decodeBotBroadcast(t, hub.fanout.(*mockBroadcaster), "test-room", EventBotCommand)
	if b != nil {
		t.Fatal("expected no broadcast for empty text")
	}
}

func TestPublishBotMessage_WithReplyTo(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "test-room", map[string]string{
		"socket-1": "user-1",
	})

	conn := newAuthedMockClient("socket-1", "user-1")
	hub.PublishBotMessage(conn, `{"room":"test-room","content":"Hello bot","replyTo":"target-1"}`)

	b := decodeBotBroadcast(t, hub.fanout.(*mockBroadcaster), "test-room", EventBotMessage)
	if b == nil {
		t.Fatal("expected bot:message broadcast")
	}
	if b.Content != "Hello bot" {
		t.Errorf("expected content 'Hello bot', got %q", b.Content)
	}
	if b.ReplyTo != "target-1" {
		t.Errorf("expected replyTo 'target-1', got %q", b.ReplyTo)
	}
}

func TestPublishBotCommand_Unauthenticated_Denied(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "test-room", map[string]string{
		"socket-1": "user-1",
	})

	// No JWT context
	conn := newMockClient("socket-1")
	hub.PublishBotCommand(conn, `{"room":"test-room","text":"/kick"}`)

	b := decodeBotBroadcast(t, hub.fanout.(*mockBroadcaster), "test-room", EventBotCommand)
	if b != nil {
		t.Fatal("expected no broadcast for unauthenticated user")
	}
}

func TestPublishBotMessage_FallbackTextToContent(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "room-a", map[string]string{
		"sock-1": "alice",
	})

	conn := newAuthedMockClient("sock-1", "alice")
	hub.PublishBotMessage(conn, `{"room":"room-a","content":"hello via content"}`)

	b := decodeBotBroadcast(t, hub.fanout.(*mockBroadcaster), "room-a", EventBotMessage)
	if b == nil {
		t.Fatal("expected broadcast")
	}
	if b.Content != "hello via content" {
		t.Errorf("expected 'hello via content', got %q", b.Content)
	}
}

func TestPublishBotMessage_BotWithPermissions(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "room-b", map[string]string{
		"bot-sock": "helper-bot",
	})

	conn := &mockClient{
		id:     "bot-sock",
		claims: &pkg.Claims{
			Username:    "helper-bot",
			Role:        "user",
			Permissions: []string{"signal:kick"},
		},
	}

	hub.PublishBotMessage(conn, `{"room":"room-b","content":"bot says hi"}`)

	b := decodeBotBroadcast(t, hub.fanout.(*mockBroadcaster), "room-b", EventBotMessage)
	if b == nil {
		t.Fatal("expected broadcast from bot")
	}
	if b.Content != "bot says hi" {
		t.Errorf("expected 'bot says hi', got %q", b.Content)
	}
	if b.From.Identity != "helper-bot" {
		t.Errorf("expected identity 'helper-bot', got %q", b.From.Identity)
	}
}

func TestPublishBotCommand_DomainRoom_BroadcastsToCompoundKey(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "domain-a:test-room", map[string]string{
		"socket-1": "user-1",
	})

	conn := newAuthedMockClient("socket-1", "user-1")
	hub.PublishBotCommand(conn, `{"room":"test-room","domain_uuid":"domain-a","text":"/kick alice"}`)

	b := decodeBotBroadcast(t, hub.fanout.(*mockBroadcaster), "domain-a:test-room", EventBotCommand)
	if b == nil {
		t.Fatal("expected bot:command broadcast on compound key")
	}
	if b.Room != "test-room" {
		t.Errorf("expected logical room 'test-room', got %q", b.Room)
	}
}
