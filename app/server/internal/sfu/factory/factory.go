package factory

import (
	"GOSpeak/internal/config"
	"fmt"

	"GOSpeak/internal/sfu"
	"GOSpeak/internal/sfu/providers/livekit"
)

// NewProvider 构造当前配置对应的 SFU provider。LiveKit 是唯一支持的实现。
func NewProvider(cfg *config.Config) (sfu.Provider, error) {
	name := cfg.SFUProvider
	if name == "" {
		name = "livekit"
	}
	if name != "livekit" {
		return nil, fmt.Errorf("unknown SFU provider: %q", name)
	}
	return livekit.NewService(cfg), nil
}
