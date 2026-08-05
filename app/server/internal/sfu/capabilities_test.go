package sfu

import "testing"

func TestCapabilitiesMatrix(t *testing.T) {
	cases := []struct {
		name string
		want Capabilities
	}{
		{
			name: "livekit",
			want: Capabilities{
				ServerMute: true, ServerKick: true, DeleteRoom: true, AdminToken: true, ListRooms: true, ListMembers: true,
				MuteLevel: EnforcementHard, KickLevel: EnforcementHard, DeleteLevel: EnforcementHard,
				ListLevel: EnforcementHard, AdminLevel: EnforcementHard,
			},
		},
		{
			name: "mediasoup",
			want: Capabilities{
				ServerMute: true, ServerKick: true, DeleteRoom: true, AdminToken: false, ListRooms: true, ListMembers: true,
				MuteLevel: EnforcementHard, KickLevel: EnforcementHard, DeleteLevel: EnforcementHard,
				ListLevel: EnforcementHard, AdminLevel: EnforcementNone,
			},
		},
		{
			name: "srs",
			want: Capabilities{
				ServerMute: true, ServerKick: true, DeleteRoom: true, AdminToken: true, ListRooms: true, ListMembers: true,
				MuteLevel: EnforcementDegraded, KickLevel: EnforcementHard, DeleteLevel: EnforcementHard,
				ListLevel: EnforcementDegraded, AdminLevel: EnforcementHard,
			},
		},
		{
			name: "cloudflare",
			want: Capabilities{
				ServerMute: false, ServerKick: true, DeleteRoom: true, AdminToken: false, ListRooms: true, ListMembers: true,
				MuteLevel: EnforcementSoft, KickLevel: EnforcementHard, DeleteLevel: EnforcementDegraded,
				ListLevel: EnforcementDegraded, AdminLevel: EnforcementNone,
			},
		},
		{
			name: "daily",
			want: Capabilities{
				ServerMute: false, ServerKick: true, DeleteRoom: true, AdminToken: false, ListRooms: true, ListMembers: true,
				MuteLevel: EnforcementSoft, KickLevel: EnforcementHard, DeleteLevel: EnforcementHard,
				ListLevel: EnforcementHard, AdminLevel: EnforcementNone,
			},
		},
		{
			name: "agora",
			want: Capabilities{
				ServerMute: true, ServerKick: true, DeleteRoom: true, AdminToken: false, ListRooms: true, ListMembers: true,
				MuteLevel: EnforcementDegraded, KickLevel: EnforcementDegraded, DeleteLevel: EnforcementHard,
				ListLevel: EnforcementHard, AdminLevel: EnforcementNone,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CapabilitiesFor(tc.name)
			if got != tc.want {
				t.Fatalf("CapabilitiesFor(%q) = %+v, want %+v", tc.name, got, tc.want)
			}
			if got.ServerMute != LevelEnabled(got.MuteLevel) {
				t.Fatalf("ServerMute=%v but MuteLevel=%q", got.ServerMute, got.MuteLevel)
			}
			if got.ServerKick != LevelEnabled(got.KickLevel) {
				t.Fatalf("ServerKick=%v but KickLevel=%q", got.ServerKick, got.KickLevel)
			}
			if got.DeleteRoom != (got.DeleteLevel != EnforcementNone) {
				t.Fatalf("DeleteRoom=%v but DeleteLevel=%q", got.DeleteRoom, got.DeleteLevel)
			}
			if got.ListRooms != (got.ListLevel != EnforcementNone) || got.ListMembers != (got.ListLevel != EnforcementNone) {
				t.Fatalf("ListRooms/ListMembers=%v/%v but ListLevel=%q", got.ListRooms, got.ListMembers, got.ListLevel)
			}
			if got.AdminToken != (got.AdminLevel == EnforcementHard) {
				t.Fatalf("AdminToken=%v but AdminLevel=%q", got.AdminToken, got.AdminLevel)
			}
		})
	}
}

func TestCapabilitiesForUnknownProviderDefaultsToNone(t *testing.T) {
	got := CapabilitiesFor("unknown")
	want := Capabilities{
		MuteLevel: EnforcementNone, KickLevel: EnforcementNone, DeleteLevel: EnforcementNone,
		ListLevel: EnforcementNone, AdminLevel: EnforcementNone,
	}
	if got != want {
		t.Fatalf("CapabilitiesFor(unknown) = %+v, want %+v", got, want)
	}
}

func TestAllProviderCapabilitiesCoversKnownProviders(t *testing.T) {
	all := AllProviderCapabilities()
	for _, name := range []string{"livekit", "agora", "srs", "cloudflare", "daily", "mediasoup"} {
		if _, ok := all[name]; !ok {
			t.Fatalf("missing provider %s", name)
		}
	}
}
