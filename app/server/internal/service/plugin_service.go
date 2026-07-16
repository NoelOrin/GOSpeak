package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/plugin"
)

type PluginService struct {
	reg *plugin.Registry
}

func NewPluginService(reg *plugin.Registry) *PluginService {
	return &PluginService{reg: reg}
}

func (s *PluginService) List() []plugin.Info {
	list := s.reg.List()
	for i := range list {
		list[i].Config = redactConfig(list[i].Config)
	}
	return list
}

func (s *PluginService) Get(name string) (*plugin.Info, error) {
	for _, info := range s.reg.List() {
		if info.Name == name {
			cp := info
			cp.Config = redactConfig(cp.Config)
			return &cp, nil
		}
	}
	return nil, pkg.NewAppError(pkg.NOT_FOUND, "plugin not found")
}

type UpdatePluginConfigInput struct {
	Name    string         `json:"name" binding:"required"`
	Enabled *bool          `json:"enabled"`
	Config  map[string]any `json:"config"`
	// Restart 配置变更后是否重启插件（默认 true）
	Restart *bool `json:"restart"`
}

func (s *PluginService) Update(input UpdatePluginConfigInput) (*plugin.Info, error) {
	p, ok := s.reg.Get(input.Name)
	if !ok {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "plugin not found")
	}

	enabled, cfg, err := s.load(input.Name)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	if input.Config != nil {
		merged := mergeSensitiveConfig(cfg, input.Config)
		if c, ok := p.(plugin.Configurable); ok {
			norm, err := c.ValidateConfig(merged)
			if err != nil {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, err.Error())
			}
			cfg = norm
			if err := c.OnConfigUpdated(norm); err != nil {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, err.Error())
			}
		} else {
			cfg = merged
		}
	}

	host := s.reg.Host()
	if err := host.SaveConfig(input.Name, enabled, cfg); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	shouldRestart := true
	if input.Restart != nil {
		shouldRestart = *input.Restart
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if shouldRestart {
		_ = s.reg.StopOne(ctx, input.Name)
		if enabled {
			if err := s.reg.StartOne(ctx, input.Name); err != nil {
				return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, fmt.Sprintf("restart plugin failed: %v", err))
			}
		}
	} else if !enabled {
		_ = s.reg.StopOne(ctx, input.Name)
	}

	return s.Get(input.Name)
}

func (s *PluginService) load(name string) (bool, map[string]any, error) {
	return s.reg.Host().LoadConfig(name)
}

// redactConfig 隐藏敏感字段（api_key 等），避免管理端接口回传明文密钥。
func redactConfig(cfg map[string]any) map[string]any {
	if cfg == nil {
		return map[string]any{}
	}
	out := map[string]any{}
	for k, v := range cfg {
		out[k] = v
	}
	raw, ok := out["llm_providers"]
	if !ok {
		return out
	}
	arr, ok := raw.([]any)
	if !ok {
		// 可能是 []map 经 json roundtrip 后为 []any；再尝试编码解码
		b, err := json.Marshal(raw)
		if err != nil {
			return out
		}
		var tmp []map[string]any
		if err := json.Unmarshal(b, &tmp); err != nil {
			return out
		}
		redacted := make([]any, 0, len(tmp))
		for _, item := range tmp {
			cp := map[string]any{}
			for k, v := range item {
				cp[k] = v
			}
			if key, ok := cp["api_key"].(string); ok && key != "" {
				cp["api_key"] = ""
				cp["api_key_set"] = true
			} else {
				cp["api_key_set"] = false
			}
			redacted = append(redacted, cp)
		}
		out["llm_providers"] = redacted
		return out
	}
	redacted := make([]any, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			redacted = append(redacted, item)
			continue
		}
		cp := map[string]any{}
		for k, v := range m {
			cp[k] = v
		}
		if key, ok := cp["api_key"].(string); ok && key != "" {
			cp["api_key"] = ""
			cp["api_key_set"] = true
		} else {
			cp["api_key_set"] = false
		}
		redacted = append(redacted, cp)
	}
	out["llm_providers"] = redacted
	return out
}

// mergeSensitiveConfig 保留旧配置中的敏感字段（如 api_key），当新值为空时不覆盖。
func mergeSensitiveConfig(oldCfg, newCfg map[string]any) map[string]any {
	if newCfg == nil {
		return oldCfg
	}
	out := map[string]any{}
	for k, v := range newCfg {
		out[k] = v
	}
	oldProviders := asMapSlice(oldCfg["llm_providers"])
	newProviders := asMapSlice(out["llm_providers"])
	if oldProviders == nil || newProviders == nil {
		return out
	}
	oldByName := map[string]map[string]any{}
	for _, p := range oldProviders {
		if name, _ := p["name"].(string); name != "" {
			oldByName[name] = p
		}
	}
	for i, p := range newProviders {
		name, _ := p["name"].(string)
		old := oldByName[name]
		if old == nil {
			continue
		}
		newKey, _ := p["api_key"].(string)
		oldKey, _ := old["api_key"].(string)
		if strings.TrimSpace(newKey) == "" && strings.TrimSpace(oldKey) != "" {
			p["api_key"] = oldKey
			newProviders[i] = p
		}
	}
	out["llm_providers"] = newProviders
	return out
}

func asMapSlice(v any) []map[string]any {
	if v == nil {
		return nil
	}
	if arr, ok := v.([]map[string]any); ok {
		return arr
	}
	if arr, ok := v.([]any); ok {
		out := make([]map[string]any, 0, len(arr))
		for _, item := range arr {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out []map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return out
}
