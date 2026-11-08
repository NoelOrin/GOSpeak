package livekit

import (
	"reflect"
	"testing"

	"GOSpeak/internal/pkg"
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
	if !reflect.DeepEqual(caps, sfu.CapabilitiesFor("livekit")) {
		t.Fatalf("Capabilities() = %+v, want %+v", caps, sfu.CapabilitiesFor("livekit"))
	}
}

func TestUnconfiguredOperations(t *testing.T) {
	svc := &Service{}
	if err := svc.MuteParticipant("room", "user", "track", true); err == nil {
		t.Fatal("expected MuteParticipant error when unconfigured")
	} else if appErr, ok := err.(*pkg.AppError); !ok || appErr.Code != pkg.SFU_NOT_CONFIGURED {
		t.Fatalf("MuteParticipant error = %v, want SFU_NOT_CONFIGURED", err)
	}
	if err := svc.RemoveParticipant("room", "user"); err == nil {
		t.Fatal("expected RemoveParticipant error when unconfigured")
	} else if appErr, ok := err.(*pkg.AppError); !ok || appErr.Code != pkg.SFU_NOT_CONFIGURED {
		t.Fatalf("RemoveParticipant error = %v, want SFU_NOT_CONFIGURED", err)
	}
	if err := svc.DeleteRoom("room"); err == nil {
		t.Fatal("expected DeleteRoom error when unconfigured")
	} else if appErr, ok := err.(*pkg.AppError); !ok || appErr.Code != pkg.SFU_NOT_CONFIGURED {
		t.Fatalf("DeleteRoom error = %v, want SFU_NOT_CONFIGURED", err)
	}
}
