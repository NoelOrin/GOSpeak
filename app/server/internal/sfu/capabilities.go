package sfu

// CapabilitiesFor returns the declarative media-layer capability matrix for a provider.
// Keep this in sync with each provider's method implementations.
func CapabilitiesFor(provider string) Capabilities {
	if provider == "" || provider == DefaultProviderName {
		return Capabilities{
			ServerMute: true, ServerKick: true, DeleteRoom: true, ListRooms: true, ListMembers: true,
			MuteLevel: EnforcementHard, KickLevel: EnforcementHard, DeleteLevel: EnforcementHard,
			ListLevel: EnforcementHard,
		}
	}
	return Capabilities{
		MuteLevel: EnforcementNone, KickLevel: EnforcementNone, DeleteLevel: EnforcementNone,
		ListLevel: EnforcementNone,
	}
}

// AllProviderCapabilities returns capability matrices for every known provider.
func AllProviderCapabilities() map[string]Capabilities {
	return map[string]Capabilities{
		DefaultProviderName: CapabilitiesFor(DefaultProviderName),
	}
}
