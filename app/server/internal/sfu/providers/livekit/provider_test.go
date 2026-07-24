package livekit

import (
	"testing"

	"GOSpeak/internal/sfu"
)

var _ sfu.Provider = (*Service)(nil)

func TestProviderName(t *testing.T) {
	if got := (&Service{}).ProviderName(); got != "livekit" {
		t.Fatalf("ProviderName = %q, want livekit", got)
	}
}

func TestCapabilities(t *testing.T) {
	caps := (&Service{}).Capabilities()
	if caps.ServerKick != true {
		t.Fatalf("ServerKick=%v, want true", caps.ServerKick)
	}
	if caps.ServerMute != true {
		t.Fatalf("ServerMute=%v, want true", caps.ServerMute)
	}
}
