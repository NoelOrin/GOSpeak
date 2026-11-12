package sfu

import "testing"

func TestPermanentMuteTTLExceeds24h(t *testing.T) {
	if PermanentMuteTTLSeconds <= 24*60*60 {
		t.Fatalf("permanent mute ttl %d must exceed 24h", PermanentMuteTTLSeconds)
	}
}
