package botbase

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/plugin"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const Name = "bot-base"

// Plugin bot 基础模块：作为后端基础插件挂载。
// - 注册主 API 下的插件路由
// - 可按配置自启 side server（小服务端）供 bot 进程/工具调用
// - 配置支持多 LLM 供应商（为后续 manage 配置页预留）
type Plugin struct {
	host   plugin.Host
	log    *logrus.Entry
	mu     sync.Mutex
	cfg    Config
	side   plugin.SideServer
	cancel context.CancelFunc
}

type Config struct {
	// SideServer 可选自启小服务
	SideServer SideServerConfig `json:"side_server"`
	// LLMProviders 多供应商大模型配置
	LLMProviders []LLMProviderConfig `json:"llm_providers"`
	// DefaultProvider 默认供应商 name
	DefaultProvider string `json:"default_provider"`
}

type SideServerConfig struct {
	Enabled bool `json:"enabled"`
	// Addr 如 127.0.0.1:9200；空则随机端口
	Addr string `json:"addr"`
}

type LLMProviderConfig struct {
	Name    string `json:"name"`
	Display string `json:"display_name"`
	// Protocol: openai-compatible | anthropic | custom-http | ollama | gemini | gemini-response
	Protocol string `json:"protocol"`
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	Enabled  bool   `json:"enabled"`
	// Extra 协议扩展字段（temperature 默认值、headers 等）
	Extra   map[string]any    `json:"extra,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

func New() *Plugin {
	cfg, err := loadEmbeddedDefaultConfig()
	if err != nil {
		// 兜底：embed 异常时仍可启动
		cfg = Config{
			SideServer: SideServerConfig{Enabled: false, Addr: "127.0.0.1:9200"},
			LLMProviders: []LLMProviderConfig{
				{
					Name:     "openai",
					Display:  "OpenAI",
					Protocol: "openai-compatible",
					BaseURL:  "https://api.openai.com/v1",
					Model:    "",
					Enabled:  false,
				},
				{
					Name:     "gemini",
					Display:  "Google Gemini",
					Protocol: "gemini-response",
					BaseURL:  "https://generativelanguage.googleapis.com/v1beta",
					Model:    "",
					Enabled:  false,
				},
			},
			DefaultProvider: "",
		}
	}
	return &Plugin{cfg: cfg}
}

func (p *Plugin) Meta() plugin.Meta {
	displayName := "Bot 基础模块"
	version := "0.1.0"
	author := "gospeak"
	desc := "Bot 基础能力插件：多组件挂载入口、可选 side server、多供应商大模型配置（二进制内嵌，后端启动同步拉起）"
	if m, err := loadEmbeddedManifest(); err == nil {
		if v, ok := m["display_name"].(string); ok && v != "" {
			displayName = v
		}
		if v, ok := m["version"].(string); ok && v != "" {
			version = v
		}
		if v, ok := m["author"].(string); ok && v != "" {
			author = v
		}
		if v, ok := m["desc"].(string); ok && v != "" {
			desc = v
		}
	}
	return plugin.Meta{
		Name:        Name,
		DisplayName: displayName,
		Version:     version,
		Author:      author,
		Desc:        desc,
		Kind:        plugin.KindBuiltin,
		ConfigSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"side_server": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"enabled": map[string]any{"type": "boolean"},
						"addr":    map[string]any{"type": "string"},
					},
				},
				"default_provider": map[string]any{"type": "string"},
				"llm_providers": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name":         map[string]any{"type": "string"},
							"display_name": map[string]any{"type": "string"},
							"protocol": map[string]any{
								"type": "string",
								"enum": []string{"openai-compatible", "anthropic", "custom-http", "ollama", "gemini", "gemini-response"},
							},
							"base_url": map[string]any{"type": "string"},
							"api_key":  map[string]any{"type": "string"},
							"model":    map[string]any{"type": "string"},
							"enabled":  map[string]any{"type": "boolean"},
						},
						"required": []string{"name", "protocol"},
					},
				},
			},
		},
	}
}

func (p *Plugin) Init(host plugin.Host) error {
	p.host = host
	p.log = host.Logger(Name)

	// 加载已有配置
	_, raw, err := host.LoadConfig(Name)
	if err != nil {
		return err
	}
	if len(raw) > 0 {
		cfg, err := p.ValidateConfig(raw)
		if err != nil {
			p.log.Warnf("invalid stored config, using defaults: %v", err)
		} else {
			p.applyConfig(cfg)
		}
	} else {
		// 首次写入内嵌默认配置，并默认启用（后端启动同步拉起）
		_ = host.SaveConfig(Name, true, p.configMap())
		p.log.Info("seeded embedded default config; plugin enabled")
	}

	// 主 API 插件路由：/api/v1/plugins/bot-base/*
	host.RegisterHTTP(func(r *gin.RouterGroup) {
		r.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"ok":               true,
				"plugin":           Name,
				"side_server":      p.sideInfo(),
				"provider_count":   len(p.cfg.LLMProviders),
				"default_provider": p.cfg.DefaultProvider,
			})
		})
		r.GET("/llm/providers", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"providers": p.publicProviders()})
		})
		r.POST("/llm/models", p.handleListModels)
	})

	return nil
}

func (p *Plugin) Start(ctx context.Context) error {
	p.mu.Lock()
	cfg := p.cfg
	p.mu.Unlock()
	p.log.Infof("starting embedded bot-base plugin (assets=%v)", HasEmbeddedAssets())

	if !cfg.SideServer.Enabled {
		p.log.Info("side server disabled")
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "plugin": Name})
	})
	mux.HandleFunc("/llm/providers", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"providers": p.publicProviders()})
	})
	mux.HandleFunc("/llm/models", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		var req listModelsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		models, err := p.resolveAndListModels(r.Context(), req)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"models": models})
	})

	addr := cfg.SideServer.Addr
	if strings.TrimSpace(addr) == "" {
		addr = "127.0.0.1:0"
	}
	ss, err := p.host.StartSideServer("bot-base-http", addr, mux)
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.side = ss
	p.mu.Unlock()
	p.log.Infof("bot-base side server: %s", ss.URL())
	return nil
}

func (p *Plugin) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	p.side = nil
	return nil
}

// ValidateConfig implements plugin.Configurable
func (p *Plugin) ValidateConfig(raw map[string]any) (map[string]any, error) {
	raw = decryptConfigSecrets(raw)
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	// normalize providers
	seen := map[string]struct{}{}
	out := make([]LLMProviderConfig, 0, len(cfg.LLMProviders))
	for _, pr := range cfg.LLMProviders {
		pr.Name = strings.TrimSpace(pr.Name)
		if pr.Name == "" {
			return nil, fmt.Errorf("llm provider name is required")
		}
		if _, ok := seen[pr.Name]; ok {
			return nil, fmt.Errorf("duplicate llm provider: %s", pr.Name)
		}
		seen[pr.Name] = struct{}{}
		pr.Protocol = strings.TrimSpace(pr.Protocol)
		switch pr.Protocol {
		case "openai-compatible", "anthropic", "custom-http", "ollama", "gemini", "gemini-response":
		case "":
			return nil, fmt.Errorf("provider %s: protocol is required", pr.Name)
		default:
			return nil, fmt.Errorf("provider %s: unsupported protocol %q", pr.Name, pr.Protocol)
		}
		if pr.Display == "" {
			pr.Display = pr.Name
		}
		out = append(out, pr)
	}
	cfg.LLMProviders = out
	if cfg.DefaultProvider != "" {
		if _, ok := seen[cfg.DefaultProvider]; !ok && len(seen) > 0 {
			return nil, fmt.Errorf("default_provider %q not found", cfg.DefaultProvider)
		}
	}
	// re-marshal for stable map
	nb, _ := json.Marshal(cfg)
	m := map[string]any{}
	_ = json.Unmarshal(nb, &m)
	return encryptConfigSecrets(m)
}

func (p *Plugin) OnConfigUpdated(cfg map[string]any) error {
	norm, err := p.ValidateConfig(cfg)
	if err != nil {
		return err
	}
	p.applyConfig(norm)
	// side server 热更新：若开关变化，重启插件侧服务
	// 由 Registry/Service 层负责 stop/start；此处仅更新内存配置
	return nil
}

func (p *Plugin) applyConfig(m map[string]any) {
	m = decryptConfigSecrets(m)
	b, _ := json.Marshal(m)
	var cfg Config
	_ = json.Unmarshal(b, &cfg)
	p.mu.Lock()
	p.cfg = cfg
	p.mu.Unlock()
}

func (p *Plugin) configMap() map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	b, _ := json.Marshal(p.cfg)
	m := map[string]any{}
	_ = json.Unmarshal(b, &m)
	encrypted, _ := encryptConfigSecrets(m)
	return encrypted
}

func (p *Plugin) publicProviders() []map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]map[string]any, 0, len(p.cfg.LLMProviders))
	for _, pr := range p.cfg.LLMProviders {
		item := map[string]any{
			"name":         pr.Name,
			"display_name": pr.Display,
			"protocol":     pr.Protocol,
			"base_url":     pr.BaseURL,
			"model":        pr.Model,
			"enabled":      pr.Enabled,
			// 不回传明文 key；仅告知是否已配置
			"api_key_set": strings.TrimSpace(pr.APIKey) != "",
		}
		out = append(out, item)
	}
	return out
}

func (p *Plugin) sideInfo() map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.side == nil {
		return map[string]any{"enabled": p.cfg.SideServer.Enabled, "running": false}
	}
	return map[string]any{
		"enabled": p.cfg.SideServer.Enabled,
		"running": true,
		"url":     p.side.URL(),
		"addr":    p.side.Addr(),
	}
}

// 确保接口实现
var (
	_ plugin.Plugin       = (*Plugin)(nil)
	_ plugin.Configurable = (*Plugin)(nil)
)

type listModelsRequest struct {
	// Provider 已保存供应商 name；若提供则优先用已保存 base_url/protocol/api_key
	Provider string `json:"provider"`
	// 也可直接传草稿字段（编辑未保存时）
	Protocol string `json:"protocol"`
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
	// Model 当前选择，仅用于回传；不参与列表查询
	Model string `json:"model"`
}

func (p *Plugin) handleListModels(c *gin.Context) {
	var req listModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	models, err := p.resolveAndListModels(c.Request.Context(), req)
	if err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	pkg.Success(c, gin.H{"models": models})
}

func (p *Plugin) resolveAndListModels(ctx context.Context, req listModelsRequest) ([]string, error) {
	protocol := strings.TrimSpace(req.Protocol)
	baseURL := strings.TrimSpace(req.BaseURL)
	apiKey := strings.TrimSpace(req.APIKey)

	if name := strings.TrimSpace(req.Provider); name != "" {
		p.mu.Lock()
		var found *LLMProviderConfig
		for i := range p.cfg.LLMProviders {
			if p.cfg.LLMProviders[i].Name == name {
				found = &p.cfg.LLMProviders[i]
				break
			}
		}
		if found != nil {
			if protocol == "" {
				protocol = found.Protocol
			}
			if baseURL == "" {
				baseURL = found.BaseURL
			}
			if apiKey == "" {
				apiKey = found.APIKey
			}
		}
		p.mu.Unlock()
	}

	return listRemoteModels(ctx, listModelsInput{
		Protocol: protocol,
		BaseURL:  baseURL,
		APIKey:   apiKey,
	})
}
