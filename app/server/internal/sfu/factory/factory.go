package factory

import (
	"GOSpeak/internal/config"
	"fmt"

	"GOSpeak/internal/sfu/providers/agora"
	"GOSpeak/internal/sfu/providers/cloudflare"
	"GOSpeak/internal/sfu/providers/livekit"
	"GOSpeak/internal/sfu"
	"GOSpeak/internal/sfu/providers/srs"
)

func NewProvider(cfg *config.Config) (sfu.Provider, error) {
	// MediaSoup/Daily 已禁用保留：实现仍在 internal/sfu/providers/，但不注册。
	name := cfg.SFUProvider
	if name == "" {
		name = "livekit"
	}
	switch name {
	case "livekit":
		return livekit.NewService(cfg), nil
	case "agora":
		return agora.NewService(cfg), nil
	case "srs":
		return srs.NewService(cfg), nil
	case "cloudflare":
		return cloudflare.NewService(cfg), nil
	default:
		return nil, fmt.Errorf("unknown SFU provider: %q", name)
	}
}
