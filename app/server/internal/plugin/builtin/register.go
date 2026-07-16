package builtin

import (
	"fmt"

	"GOSpeak/internal/plugin"
	"GOSpeak/internal/plugin/builtin/botbase"
)

// RegisterAll 注册所有内置基础插件。
// 这些插件随 Go 二进制编译/嵌入，启动后端时由 Registry 同步拉起。
func RegisterAll(reg *plugin.Registry) error {
	builtins := []plugin.Plugin{
		botbase.New(),
	}
	for _, p := range builtins {
		if err := reg.Register(p); err != nil {
			return fmt.Errorf("register builtin plugin %s: %w", p.Meta().Name, err)
		}
	}
	return nil
}

// EmbeddedSummary 返回内置插件嵌入状态，便于启动日志与健康检查。
func EmbeddedSummary() []map[string]any {
	return []map[string]any{
		{
			"name":      botbase.Name,
			"embedded":  botbase.HasEmbeddedAssets(),
			"auto_start": true,
		},
	}
}
