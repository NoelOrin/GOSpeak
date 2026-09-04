package sfu

// CapabilitiesFor returns the declarative media-layer capability matrix for a provider.
// Keep this in sync with each provider's method implementations.
func CapabilitiesFor(provider string) Capabilities {
	switch provider {
	case "livekit":
		return Capabilities{
			ServerMute: true, ServerKick: true, DeleteRoom: true, ListRooms: true, ListMembers: true,
			MuteLevel: EnforcementHard, KickLevel: EnforcementHard, DeleteLevel: EnforcementHard,
			ListLevel: EnforcementHard,
		}
	// 已禁用保留：mediasoup/daily 实现仍在仓库，但不参与 provider 注册。
	case "mediasoup":
		return Capabilities{
			ServerMute: true, ServerKick: true, DeleteRoom: true, ListRooms: true, ListMembers: true,
			MuteLevel: EnforcementHard, KickLevel: EnforcementHard, DeleteLevel: EnforcementHard,
			ListLevel: EnforcementHard,
		}
	case "srs":
		// Mute degraded: 不踢流（Discord 式，成员仍可听），订阅端静音由 member:muted 事件驱动，
		// on_publish 禁推黑名单兜底（断流后无法重推）。List hard: SRS API 直查 + stream→room 反查。
		return Capabilities{
			ServerMute: true, ServerKick: true, DeleteRoom: true, ListRooms: true, ListMembers: true,
			MuteLevel: EnforcementDegraded, KickLevel: EnforcementHard, DeleteLevel: EnforcementHard,
			ListLevel: EnforcementHard,
		}
	case "cloudflare":
		// Mute degraded: CloseTracks 只关本地发布轨道（保留订阅，还能听）。Unmute soft (client re-publish)。
		return Capabilities{
			ServerMute: true, ServerKick: true, DeleteRoom: true, ListRooms: true, ListMembers: true,
			MuteLevel: EnforcementDegraded, KickLevel: EnforcementHard, DeleteLevel: EnforcementDegraded,
			ListLevel: EnforcementDegraded,
		}
	case "daily":
		return Capabilities{
			ServerMute: false, ServerKick: true, DeleteRoom: true, ListRooms: true, ListMembers: true,
			MuteLevel: EnforcementSoft, KickLevel: EnforcementHard, DeleteLevel: EnforcementHard,
			ListLevel: EnforcementHard,
		}
	case "agora":
		// Kick/mute via kicking-rule (degraded hard).
		return Capabilities{
			ServerMute: true, ServerKick: true, DeleteRoom: true, ListRooms: true, ListMembers: true,
			MuteLevel: EnforcementDegraded, KickLevel: EnforcementDegraded, DeleteLevel: EnforcementHard,
			ListLevel: EnforcementHard,
		}
	default:
		return Capabilities{
			MuteLevel: EnforcementNone, KickLevel: EnforcementNone, DeleteLevel: EnforcementNone,
			ListLevel: EnforcementNone,
		}
	}
}

// AllProviderCapabilities returns capability matrices for every known provider.
func AllProviderCapabilities() map[string]Capabilities {
	names := []string{"livekit", "agora", "srs", "cloudflare", "daily", "mediasoup"}
	out := make(map[string]Capabilities, len(names))
	for _, name := range names {
		out[name] = CapabilitiesFor(name)
	}
	return out
}
