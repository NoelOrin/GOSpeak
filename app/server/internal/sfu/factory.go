package sfu

import (
	"GOSpeak/internal/agora"
	"GOSpeak/internal/config"
	"GOSpeak/internal/daily"
	"GOSpeak/internal/livekit"
	"GOSpeak/internal/mediasoup"
	"GOSpeak/internal/srs"
	"fmt"
)

// NewProvider directly constructs the SFU provider based on config.
// All providers built in, selected at runtime.
// Supported: "livekit", "agora", "mediasoup", "srs", "daily".
func NewProvider(cfg *config.Config) (Provider, error) {
	name := cfg.SFUProvider
	if name == "" {
		name = "livekit"
	}
	switch name {
	case "livekit":
		return livekit.NewService(cfg), nil
	case "agora":
		return agora.NewService(cfg), nil
	case "mediasoup":
		return mediasoup.NewService(cfg), nil
	case "srs":
		return srs.NewService(cfg), nil
	case "daily":
		return daily.NewService(cfg), nil
	default:
		return nil, fmt.Errorf("unknown SFU provider: %q", name)
	}
}
