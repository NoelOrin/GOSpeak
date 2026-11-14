package factory

import (
	"fmt"
	"sort"

	"GOSpeak/internal/config"
	"GOSpeak/internal/sfu"
	"GOSpeak/internal/sfu/providers/livekit"
)

// ProviderFactory 由各 provider 实现的构造器。
type ProviderFactory func(cfg *config.Config) (sfu.Provider, error)

var registry = map[string]ProviderFactory{}

// Register 注册 provider 构造器；重复注册或空名视为编程错误。
func Register(name string, build ProviderFactory) {
	if name == "" {
		panic("factory: provider name must not be empty")
	}
	if _, dup := registry[name]; dup {
		panic("factory: provider registered twice: " + name)
	}
	registry[name] = build
}

// RegisteredProviders 返回已注册 provider 名单（稳定排序）。
func RegisteredProviders() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func init() {
	Register(sfu.DefaultProviderName, func(cfg *config.Config) (sfu.Provider, error) {
		return livekit.NewService(cfg), nil
	})
}

// NewProvider 构造当前配置对应的 SFU provider；未指定时回退默认 provider。
func NewProvider(cfg *config.Config) (sfu.Provider, error) {
	name := cfg.SFUProvider
	if name == "" {
		name = sfu.DefaultProviderName
	}
	build, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown SFU provider: %q (registered: %v)", name, RegisteredProviders())
	}
	return build(cfg)
}
