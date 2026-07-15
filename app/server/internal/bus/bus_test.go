package bus

import (
	"encoding/json"
	"testing"
)

func TestSubjectHelpers(t *testing.T) {
	if got := NamespaceSubject("gospeak"); got != "gospeak.signal.namespace" {
		t.Fatalf("NamespaceSubject = %q", got)
	}
	if got := RoomSubject("gospeak", "lobby"); got != "gospeak.signal.room.lobby" {
		t.Fatalf("RoomSubject = %q", got)
	}
}

func TestNewEnvelopeRoundTrip(t *testing.T) {
	payload := map[string]any{"identity": "alice"}
	env, err := NewEnvelope("inst-1", "room", "lobby", "member:joined", payload)
	if err != nil {
		t.Fatal(err)
	}
	if env.V != 1 || env.InstanceID != "inst-1" || env.Scope != "room" || env.Room != "lobby" || env.Event != "member:joined" {
		t.Fatalf("envelope fields wrong: %+v", env)
	}
	if env.TS <= 0 {
		t.Fatal("ts should be set")
	}
	var got map[string]any
	if err := json.Unmarshal(env.Payload, &got); err != nil {
		t.Fatal(err)
	}
	if got["identity"] != "alice" {
		t.Fatalf("payload = %#v", got)
	}
}
