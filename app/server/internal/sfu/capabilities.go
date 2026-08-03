package sfu

// CapabilitiesFor returns the declarative media-layer capability matrix for a provider.
// Keep this in sync with each provider's method implementations.
func CapabilitiesFor(provider string) Capabilities {
	switch provider {
	case "livekit":
		return Capabilities{
			ServerMute: true, ServerKick: true, DeleteRoom: true, AdminToken: true, ListRooms: true, ListMembers: true,
			MuteLevel: EnforcementHard, KickLevel: EnforcementHard, DeleteLevel: EnforcementHard,
			ListLevel: EnforcementHard, AdminLevel: EnforcementHard,
		}
	// 已禁用保留：mediasoup/daily 实现仍在仓库，但不参与 provider 注册。
	case "mediasoup":
		return Capabilities{
			ServerMute: true, ServerKick: true, DeleteRoom: true, AdminToken: false, ListRooms: true, ListMembers: true,
			MuteLevel: EnforcementHard, KickLevel: EnforcementHard, DeleteLevel: EnforcementHard,
			ListLevel: EnforcementHard, AdminLevel: EnforcementNone,
		}
	case "srs":
		// Mute degraded: force-unpublish via KickByStreams. Unmute soft (client re-publish).
		return Capabilities{
			ServerMute: true, ServerKick: true, DeleteRoom: true, AdminToken: true, ListRooms: true, ListMembers: true,
			MuteLevel: EnforcementDegraded, KickLevel: EnforcementHard, DeleteLevel: EnforcementHard,
			ListLevel: EnforcementDegraded, AdminLevel: EnforcementHard,
		}
	case "cloudflare":
		return Capabilities{
			ServerMute: false, ServerKick: true, DeleteRoom: true, AdminToken: false, ListRooms: true, ListMembers: true,
			MuteLevel: EnforcementSoft, KickLevel: EnforcementHard, DeleteLevel: EnforcementDegraded,
			ListLevel: EnforcementDegraded, AdminLevel: EnforcementNone,
		}
	case "daily":
		return Capabilities{
			ServerMute: false, ServerKick: true, DeleteRoom: true, AdminToken: false, ListRooms: true, ListMembers: true,
			MuteLevel: EnforcementSoft, KickLevel: EnforcementHard, DeleteLevel: EnforcementHard,
			ListLevel: EnforcementHard, AdminLevel: EnforcementNone,
		}
	case "agora":
		// Kick/mute via kicking-rule (degraded hard).
		return Capabilities{
			ServerMute: true, ServerKick: true, DeleteRoom: true, AdminToken: false, ListRooms: true, ListMembers: true,
			MuteLevel: EnforcementDegraded, KickLevel: EnforcementDegraded, DeleteLevel: EnforcementHard,
			ListLevel: EnforcementHard, AdminLevel: EnforcementNone,
		}
	default:
		return Capabilities{
			MuteLevel: EnforcementNone, KickLevel: EnforcementNone, DeleteLevel: EnforcementNone,
			ListLevel: EnforcementNone, AdminLevel: EnforcementNone,
		}
	}
}

// AllProviderCapabilities returns capability matrices for every known provider.
func AllProviderCapabilities() map[string]Capabilities {
	names := []string{"livekit", "agora", "srs", "cloudflare"}
	out := make(map[string]Capabilities, len(names))
	for _, name := range names {
		out[name] = CapabilitiesFor(name)
	}
	return out
}
