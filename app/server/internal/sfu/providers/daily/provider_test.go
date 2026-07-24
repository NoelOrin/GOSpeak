package daily

import (
	"errors"
	"testing"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
)

var _ sfu.Provider = (*Service)(nil)
var _ sfu.ClientInfoProvider = (*Service)(nil)

func TestUnsupportedOperations(t *testing.T) {
	svc := &Service{}
	if _, err := svc.GenerateAdminToken(); !errors.Is(err, pkg.ErrSFUNotSupported) {
		t.Fatalf("GenerateAdminToken: want ErrSFUNotSupported, got %v", err)
	}
	if err := svc.MuteParticipant("room", "user", "", true); !errors.Is(err, pkg.ErrSFUNotSupported) {
		t.Fatalf("MuteParticipant: want ErrSFUNotSupported, got %v", err)
	}
}

func TestProviderName(t *testing.T) {
	if got := (&Service{}).ProviderName(); got != "daily" {
		t.Fatalf("ProviderName = %q, want daily", got)
	}
}

func TestCapabilities(t *testing.T) {
	caps := (&Service{}).Capabilities()
	if caps.ServerKick != true {
		t.Fatalf("ServerKick=%v, want true", caps.ServerKick)
	}
	if caps.ServerMute != false {
		t.Fatalf("ServerMute=%v, want false", caps.ServerMute)
	}
}
