package signal

import (
	"errors"
	"testing"
	"time"

	"GOSpeak/internal/protocol"
)

const typingRoomKey = "domain-a:lobby"

func newTypingTestHub(mute *mockMuteStore) (*Hub, *mockBroadcaster) {
	hub := NewHub(nil, mute, nil, nil)
	fanout := newMockBroadcaster()
	hub.fanout = fanout
	seedKickRoom(hub, typingRoomKey, map[string]string{"sock-1": "alice"})
	return hub, fanout
}

func typingCasts(fanout *mockBroadcaster) []interface{} {
	return fanout.roomCasts[typingRoomKey][EventTyping]
}

const typingStartFrame = `{"room":"lobby","domain_uuid":"domain-a","typing":true}`
const typingStopFrame = `{"room":"lobby","domain_uuid":"domain-a","typing":false}`

func TestHub_OnTyping_StartStopAndThrottle(t *testing.T) {
	hub, fanout := newTypingTestHub(newMockMuteStore())
	conn := newAuthedMockClient("sock-1", "alice")

	hub.OnTyping(conn, typingStartFrame)
	// 连续按键上报：2s 内的重复 start 不应再次广播
	hub.OnTyping(conn, typingStartFrame)
	if got := len(typingCasts(fanout)); got != 1 {
		t.Fatalf("duplicate typing start must be throttled, got %d broadcasts", got)
	}

	hub.OnTyping(conn, typingStopFrame)
	// 重复 stop 也不广播
	hub.OnTyping(conn, typingStopFrame)
	if got := len(typingCasts(fanout)); got != 2 {
		t.Fatalf("duplicate typing stop must not re-broadcast, got %d broadcasts", got)
	}

	payload, ok := typingCasts(fanout)[1].(protocol.TypingPayload)
	if !ok {
		t.Fatalf("typing payload must be TypingPayload, got %T", typingCasts(fanout)[1])
	}
	if payload.Typing != false || payload.Room != "lobby" || payload.DomainUUID != "domain-a" {
		t.Fatalf("unexpected stop payload: %#v", payload)
	}
}

func TestHub_OnTyping_HeartbeatRenewsAfterInterval(t *testing.T) {
	hub, fanout := newTypingTestHub(newMockMuteStore())
	conn := newAuthedMockClient("sock-1", "alice")

	hub.OnTyping(conn, typingStartFrame)
	// 模拟 2s 节流窗口过去：直接把该条目的 lastSent 往前拨
	hub.typingMu.Lock()
	hub.typing[typingRoomKey+":alice"].lastSent = time.Now().Add(-3 * time.Second)
	hub.typingMu.Unlock()

	hub.OnTyping(conn, typingStartFrame)
	if got := len(typingCasts(fanout)); got != 2 {
		t.Fatalf("sustained typing must send a heartbeat after the throttle window, got %d", got)
	}
}

func TestHub_OnTyping_UsesAuthenticatedIdentity(t *testing.T) {
	hub, fanout := newTypingTestHub(newMockMuteStore())
	conn := newAuthedMockClient("sock-1", "alice")

	// 客户端伪造 identity 不应生效
	hub.OnTyping(conn, `{"room":"lobby","domain_uuid":"domain-a","identity":"bob","typing":true}`)

	payload, ok := typingCasts(fanout)[0].(protocol.TypingPayload)
	if !ok {
		t.Fatalf("typing payload must be TypingPayload, got %T", typingCasts(fanout)[0])
	}
	if payload.Identity != "alice" {
		t.Fatalf("must broadcast the authenticated identity, got %v", payload.Identity)
	}
}

func TestHub_OnTyping_IgnoresNonMember(t *testing.T) {
	hub, fanout := newTypingTestHub(newMockMuteStore())
	stranger := newAuthedMockClient("sock-2", "bob")

	hub.OnTyping(stranger, typingStartFrame)
	if got := len(typingCasts(fanout)); got != 0 {
		t.Fatalf("non-member typing must be ignored, got %d broadcasts", got)
	}
}

func TestHub_OnTyping_MutedAndMuteCheckErrorFailClosed(t *testing.T) {
	muted := newMockMuteStore()
	muted.muted["alice"] = true
	hub, fanout := newTypingTestHub(muted)
	hub.OnTyping(newAuthedMockClient("sock-1", "alice"), typingStartFrame)
	if got := len(typingCasts(fanout)); got != 0 {
		t.Fatalf("muted member typing must be ignored, got %d broadcasts", got)
	}

	broken := newMockMuteStore()
	broken.setErr(errors.New("db down"))
	hub2, fanout2 := newTypingTestHub(broken)
	hub2.OnTyping(newAuthedMockClient("sock-1", "alice"), typingStartFrame)
	if got := len(typingCasts(fanout2)); got != 0 {
		t.Fatalf("typing must be ignored when mute check fails, got %d broadcasts", got)
	}
}

func TestHub_OnTyping_IgnoresMalformedFrames(t *testing.T) {
	hub, fanout := newTypingTestHub(newMockMuteStore())
	conn := newAuthedMockClient("sock-1", "alice")

	hub.OnTyping(conn, `not-json`)
	hub.OnTyping(conn, `{"domain_uuid":"domain-a","typing":true}`)
	hub.OnTyping(conn, `{"room":"other-room","domain_uuid":"domain-a","typing":true}`)
	if got := len(typingCasts(fanout)); got != 0 {
		t.Fatalf("malformed / unknown-room frames must be ignored, got %d broadcasts", got)
	}
}
