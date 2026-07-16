package sfu

import "testing"

func TestCapabilitiesForAgoraLevels(t *testing.T) {
	caps := CapabilitiesFor("agora")
	if caps.MuteLevel != EnforcementDegraded || caps.KickLevel != EnforcementDegraded {
		t.Fatalf("agora levels mute=%s kick=%s", caps.MuteLevel, caps.KickLevel)
	}
	if !caps.ServerMute || !caps.ServerKick {
		t.Fatal("bool flags should follow levels")
	}
}

func TestCapabilitiesForSRSMuteDegraded(t *testing.T) {
	caps := CapabilitiesFor("srs")
	if caps.MuteLevel != EnforcementDegraded {
		t.Fatalf("srs mute level=%s", caps.MuteLevel)
	}
}

func TestAllProviderCapabilitiesCoversKnownProviders(t *testing.T) {
	all := AllProviderCapabilities()
	for _, name := range []string{"livekit", "agora", "mediasoup", "srs", "daily", "cloudflare"} {
		if _, ok := all[name]; !ok {
			t.Fatalf("missing provider %s", name)
		}
	}
}
