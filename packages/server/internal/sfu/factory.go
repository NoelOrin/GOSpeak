package sfu

import (
	"fmt"
	"go_rtc/internal/config"
	"go_rtc/internal/livekit"
)

// NewProvider creates an SFU provider based on config.SFUProvider.
// Supported values: "livekit" (default). Returns an error for unknown providers.
func NewProvider(cfg *config.Config) (Provider, error) {
	switch cfg.SFUProvider {
	case "", "livekit":
		return livekit.NewService(cfg), nil
	default:
		return nil, fmt.Errorf("unknown SFU provider: %q", cfg.SFUProvider)
	}
}
