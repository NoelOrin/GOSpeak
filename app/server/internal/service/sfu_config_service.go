package service

import (
	"strings"

	"GOSpeak/internal/config"
	"GOSpeak/internal/logger"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/sfu"

	"gorm.io/gorm"
)

// UpdateSFUConfigRequest is the handler-facing DTO for updating SFU configuration.
type UpdateSFUConfigRequest struct {
	Provider      string `json:"provider"`
	LiveKitHost   string `json:"livekit_host"`
	LiveKitKey    string `json:"livekit_key"`
	LiveKitSecret string `json:"livekit_secret"`
}

// PublicSFUConfig 管理后台可读配置视图：密钥字段已脱敏，不回显明文。
type PublicSFUConfig struct {
	Provider         string `json:"provider"`
	LiveKitHost      string `json:"livekit_host"`
	LiveKitKey       string `json:"livekit_key"`
	LiveKitSecret    string `json:"livekit_secret"`
	LiveKitSecretSet bool   `json:"livekit_secret_set"`
	CreatedAt        string `json:"created_at,omitempty"`
	UpdatedAt        string `json:"updated_at,omitempty"`
}

type SFUConfigService struct {
	repo    *repository.SFUConfigRepository
	baseCfg *config.Config
}

// defaultSFUProvider 是 env 未指定时的回退值。
const defaultSFUProvider = sfu.DefaultProviderName

func NewSFUConfigService(repo *repository.SFUConfigRepository, baseCfg *config.Config) *SFUConfigService {
	return &SFUConfigService{repo: repo, baseCfg: baseCfg}
}

// Get 返回当前激活 provider 的配置。若该 provider 尚无 DB 记录，以 env 默认值创建。
func (s *SFUConfigService) Get() (*model.SFUConfig, error) {
	active, err := s.repo.GetActiveProvider()
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	// 兼容历史数据：旧版本可能写入已下线 provider，读路径自愈回默认并 Warn 可见。
	if !isValidProvider(active) {
		logger.WithComponent("SFU").Warnf("active provider %q is no longer valid, falling back to %s", active, defaultSFUProvider)
		active = defaultSFUProvider
		if err := s.repo.SetActiveProvider(active); err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	cfg, err := s.repo.GetByProvider(active)
	if err == nil {
		return cfg, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	// 首次运行：用 env 默认值创建该 provider 的行
	cfg = s.defaultConfigForProvider(active)
	if err := s.repo.Save(cfg); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return cfg, nil
}

// GetProviderConfig 返回指定 provider 的配置。未找到时以 env 默认值创建。
func (s *SFUConfigService) GetProviderConfig(provider string) (*model.SFUConfig, error) {
	if !isValidProvider(provider) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "invalid provider: only livekit is supported")
	}
	cfg, err := s.repo.GetByProvider(provider)
	if err == nil {
		return cfg, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	cfg = s.defaultConfigForProvider(provider)
	if err := s.repo.Save(cfg); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return cfg, nil
}

// UpdateFromDTO 更新 SFU 配置，并将其设为当前激活。
func (s *SFUConfigService) UpdateFromDTO(req *UpdateSFUConfigRequest) (*model.SFUConfig, error) {
	if !isValidProvider(req.Provider) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "provider must be livekit")
	}

	// 读取现有配置（若有），保留未在请求中发送的字段
	existing, err := s.repo.GetByProvider(req.Provider)
	cfg := &model.SFUConfig{Provider: req.Provider}
	if err == nil {
		cfg = existing
	}

	// 密钥字段为空时保留旧值，避免管理后台脱敏回写清空密钥。
	cfg.LiveKitHost = req.LiveKitHost
	cfg.LiveKitKey = req.LiveKitKey
	cfg.LiveKitSecret = pkg.KeepSecret(req.LiveKitSecret, cfg.LiveKitSecret)

	if err := s.repo.Save(cfg); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	// 更新后自动切换为该 provider 激活
	if err := s.repo.SetActiveProvider(req.Provider); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	return cfg, nil
}

// SwitchProvider 切换当前激活的 provider，不修改配置。返回新激活 provider 的配置。
func (s *SFUConfigService) SwitchProvider(provider string) (*model.SFUConfig, error) {
	if !isValidProvider(provider) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "provider must be livekit")
	}
	if err := s.repo.SetActiveProvider(provider); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	// 返回该 provider 的配置（若没有则用 env 默认值创建）
	return s.GetProviderConfig(provider)
}

// ListProviders 返回已配置 provider 的列表 + 当前激活的 provider 名称。
// 历史遗留的非 LiveKit 行会被过滤掉，避免管理后台出现已下线的 provider。
func (s *SFUConfigService) ListProviders() ([]model.SFUConfig, string, error) {
	active, err := s.repo.GetActiveProvider()
	if err != nil {
		return nil, "", pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	cfgs, err := s.repo.ListProviders()
	if err != nil {
		return nil, "", pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	filtered := make([]model.SFUConfig, 0, len(cfgs))
	for _, c := range cfgs {
		if isValidProvider(c.Provider) {
			filtered = append(filtered, c)
		}
	}
	if !isValidProvider(active) {
		active = defaultSFUProvider
	}
	return filtered, active, nil
}

// SyncFromEnv 启动时调用，将 env 配置写入 DB（仅当尚无 DB 记录时）。
// 不会覆盖用户在 DB 中已保存的配置。
func (s *SFUConfigService) SyncFromEnv() error {
	provider := defaultSFUProvider
	_, err := s.repo.GetByProvider(provider)
	if err == gorm.ErrRecordNotFound {
		// 无记录：用 env 默认值创建
		cfg := s.defaultConfigForProvider(provider)
		if err := s.repo.Save(cfg); err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	} else if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	// 设置激活 provider（env 指定的唯一支持值）
	active := s.baseCfg.SFUProvider
	if active == "" {
		active = defaultSFUProvider
	}
	if !isValidProvider(active) {
		logger.WithComponent("SFUConfig").Warnf("SFU_PROVIDER=%q unsupported, falling back to %s", active, defaultSFUProvider)
		active = defaultSFUProvider
	}
	if err := s.repo.SetActiveProvider(active); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// ResolveConfig 返回当前激活 provider 的完整运行时 config，
// 覆盖 baseCfg 中对应字段。
func (s *SFUConfigService) ResolveConfig() (*config.Config, error) {
	cfg, err := s.Get()
	if err != nil {
		return nil, err
	}
	resolved := *s.baseCfg
	resolved.SFUProvider = defaultSFUProvider

	// env 的 SFU 设置直接覆盖 DB 配置。
	// 仅当 env 显式设置了非空值时覆盖，避免用空默认值清空 DB 中已保存的值。
	if s.baseCfg.LiveKitHost == "" {
		resolved.LiveKitHost = cfg.LiveKitHost
	}
	if s.baseCfg.LiveKitKey == "" {
		resolved.LiveKitKey = cfg.LiveKitKey
	}
	if s.baseCfg.LiveKitSecret == "" {
		resolved.LiveKitSecret = cfg.LiveKitSecret
	}
	// 浏览器侧地址留空是正常形态：表示走应用内置的同源 /livekit 反代，
	// 由请求 Host 推导，不再回退到可能只有容器内网可达的 LiveKitHost。
	resolved.LiveKitPublicURL = strings.TrimSpace(s.baseCfg.LiveKitPublicURL)
	return &resolved, nil
}

func isValidProvider(p string) bool {
	return p == defaultSFUProvider
}

// defaultConfigForProvider 返回 env 默认配置。
func (s *SFUConfigService) defaultConfigForProvider(provider string) *model.SFUConfig {
	cfg := &model.SFUConfig{Provider: provider}
	cfg.LiveKitHost = s.baseCfg.LiveKitHost
	cfg.LiveKitKey = s.baseCfg.LiveKitKey
	cfg.LiveKitSecret = s.baseCfg.LiveKitSecret
	return cfg
}

// ToPublicSFUConfig 将内部配置转为管理后台安全视图。
func ToPublicSFUConfig(cfg *model.SFUConfig) *PublicSFUConfig {
	if cfg == nil {
		return nil
	}
	p := &PublicSFUConfig{
		Provider:         cfg.Provider,
		LiveKitHost:      cfg.LiveKitHost,
		LiveKitKey:       cfg.LiveKitKey,
		LiveKitSecret:    "",
		LiveKitSecretSet: cfg.LiveKitSecret != "",
	}
	if !cfg.CreatedAt.IsZero() {
		p.CreatedAt = cfg.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if !cfg.UpdatedAt.IsZero() {
		p.UpdatedAt = cfg.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	return p
}

// ToPublicSFUConfigs 批量脱敏。
func ToPublicSFUConfigs(cfgs []model.SFUConfig) []PublicSFUConfig {
	out := make([]PublicSFUConfig, 0, len(cfgs))
	for i := range cfgs {
		if pub := ToPublicSFUConfig(&cfgs[i]); pub != nil {
			out = append(out, *pub)
		}
	}
	return out
}
