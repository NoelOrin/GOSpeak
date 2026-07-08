package factory

import (
	"GOSpeak/internal/config"
	"fmt"

	"GOSpeak/internal/agora"
	"GOSpeak/internal/daily"
	"GOSpeak/internal/livekit"
	"GOSpeak/internal/mediasoup"
	"GOSpeak/internal/sfu"
	"GOSpeak/internal/srs"
)

func NewProvider(cfg *config.Config) (sfu.Provider, error) {
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
