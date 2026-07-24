package agora

import (
	"errors"
	"testing"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
)

var _ sfu.Provider = (*Service)(nil)
var _ sfu.ClientInfoProvider = (*Service)(nil)
var _ sfu.TimedMuteProvider = (*Service)(nil)
var _ sfu.MuteRuleStoreSetter = (*Service)(nil)

func TestUnsupportedOperations(t *testing.T) {
	svc := &Service{}
	if _, err := svc.GenerateAdminToken(); !errors.Is(err, pkg.ErrSFUNotSupported) {
		t.Fatalf("GenerateAdminToken: want ErrSFUNotSupported, got %v", err)
	}
}

func TestProviderName(t *testing.T) {
	if got := (&Service{}).ProviderName(); got != "agora" {
		t.Fatalf("ProviderName = %q, want agora", got)
	}
}

func TestCapabilities(t *testing.T) {
	caps := (&Service{}).Capabilities()
	if caps.MuteLevel != sfu.EnforcementDegraded || caps.KickLevel != sfu.EnforcementDegraded {
		t.Fatalf("unexpected levels: %+v", caps)
	}
}

func TestMuteRuleKey(t *testing.T) {
	if got := muteRuleKey("r", "u"); got != "r|u" {
		t.Fatalf("key=%q", got)
	}
}

func TestMuteWithoutConfigFails(t *testing.T) {
	svc := &Service{}
	err := svc.MuteParticipantTimed("room", "user", "", true, 60)
	if err == nil {
		t.Fatal("expected error when unconfigured")
	}
}
