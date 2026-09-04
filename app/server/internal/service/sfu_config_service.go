package service

import (
	"GOSpeak/internal/config"
	"GOSpeak/internal/logger"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"gorm.io/gorm"
)

// UpdateSFUConfigRequest is the handler-facing DTO for updating SFU configuration.
type UpdateSFUConfigRequest struct {
	Provider            string `json:"provider"`
	LiveKitHost         string `json:"livekit_host"`
	LiveKitKey          string `json:"livekit_key"`
	LiveKitSecret       string `json:"livekit_secret"`
	AgoraAppID          string `json:"agora_app_id"`
	AgoraAppCertificate string `json:"agora_app_certificate"`
	AgoraHost           string `json:"agora_host"`
	AgoraCustomerID     string `json:"agora_customer_id"`
	AgoraCustomerSecret string `json:"agora_customer_secret"`
	MediaSoupBridgeURL  string `json:"mediasoup_bridge_url"`
	MediaSoupHost       string `json:"mediasoup_host"`
	SRSHost             string `json:"srs_host"`
	SRSApiPort          string `json:"srs_api_port"`
	SRSSecret           string `json:"srs_secret"`
	SRSWHIPURL          string `json:"srs_whip_url"`
	SRSPublicHost       string `json:"srs_public_host"`
	DailyAPIKey         string `json:"daily_api_key"`
	DailyDomain         string `json:"daily_domain"`
	CFAppID             string `json:"cf_app_id"`
	CFAppSecret         string `json:"cf_app_secret"`
	CFStunURL           string `json:"cf_stun_url"`
}

// PublicSFUConfig 管理后台可读配置视图：密钥字段已脱敏，不回显明文。
type PublicSFUConfig struct {
	Provider               string `json:"provider"`
	LiveKitHost            string `json:"livekit_host"`
	LiveKitKey             string `json:"livekit_key"`
	LiveKitSecret          string `json:"livekit_secret"`
	LiveKitSecretSet       bool   `json:"livekit_secret_set"`
	AgoraAppID             string `json:"agora_app_id"`
	AgoraAppCertificate    string `json:"agora_app_certificate"`
	AgoraAppCertificateSet bool   `json:"agora_app_certificate_set"`
	AgoraHost              string `json:"agora_host"`
	AgoraCustomerID        string `json:"agora_customer_id"`
	AgoraCustomerSecret    string `json:"agora_customer_secret"`
	AgoraCustomerSecretSet bool   `json:"agora_customer_secret_set"`
	MediaSoupBridgeURL     string `json:"mediasoup_bridge_url"`
	MediaSoupHost          string `json:"mediasoup_host"`
	SRSHost                string `json:"srs_host"`
	SRSApiPort             string `json:"srs_api_port"`
	SRSSecret              string `json:"srs_secret"`
	SRSSecretSet           bool   `json:"srs_secret_set"`
	SRSWHIPURL             string `json:"srs_whip_url"`
	SRSPublicHost          string `json:"srs_public_host"`
	DailyAPIKey            string `json:"daily_api_key"`
	DailyAPIKeySet         bool   `json:"daily_api_key_set"`
	DailyDomain            string `json:"daily_domain"`
	CFAppID                string `json:"cf_app_id"`
	CFAppSecret            string `json:"cf_app_secret"`
	CFAppSecretSet         bool   `json:"cf_app_secret_set"`
	CFStunURL              string `json:"cf_stun_url"`
	CreatedAt              string `json:"created_at,omitempty"`
	UpdatedAt              string `json:"updated_at,omitempty"`
}

type SFUConfigService struct {
	repo    *repository.SFUConfigRepository
	baseCfg *config.Config
}

// frontendDisabledProviders 与前端 DISABLED_SFU_PROVIDERS 对齐：
// 这些 provider 仅保留代码与类型，不允许作为激活 provider。
var frontendDisabledProviders = map[string]bool{"agora": true}

// defaultSFUProvider 是 env 未指定或禁用 provider 无法激活时的回退值。
const defaultSFUProvider = "livekit"

func NewSFUConfigService(repo *repository.SFUConfigRepository, baseCfg *config.Config) *SFUConfigService {
	return &SFUConfigService{repo: repo, baseCfg: baseCfg}
}

// Get 返回当前激活 provider 的配置。若该 provider 尚无 DB 记录，以 env 默认值创建。
func (s *SFUConfigService) Get() (*model.SFUConfig, error) {
	active, err := s.repo.GetActiveProvider()
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
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
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "invalid provider: must be livekit, agora, srs, or cloudflare")
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

// UpdateFromDTO 更新指定 provider 的配置，并将其设为当前激活。
// 每个 provider 的行独立，切换时其他 provider 的配置不受影响。
func (s *SFUConfigService) UpdateFromDTO(req *UpdateSFUConfigRequest) (*model.SFUConfig, error) {
	if frontendDisabledProviders[req.Provider] {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "provider is temporarily disabled")
	}
	if !isValidProvider(req.Provider) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "provider must be livekit, agora, srs, or cloudflare")
	}

	// 读取现有配置（若有），保留未在请求中发送的字段
	existing, err := s.repo.GetByProvider(req.Provider)
	cfg := &model.SFUConfig{Provider: req.Provider}
	if err == nil {
		cfg = existing
	}

	// 仅更新当前 provider 自己的字段。
	// 密钥类字段为空时保留旧值，避免管理后台脱敏回写清空密钥。
	switch req.Provider {
	case "livekit":
		cfg.LiveKitHost = req.LiveKitHost
		cfg.LiveKitKey = req.LiveKitKey
		cfg.LiveKitSecret = pkg.KeepSecret(req.LiveKitSecret, cfg.LiveKitSecret)
	case "agora":
		cfg.AgoraAppID = req.AgoraAppID
		cfg.AgoraAppCertificate = pkg.KeepSecret(req.AgoraAppCertificate, cfg.AgoraAppCertificate)
		cfg.AgoraHost = req.AgoraHost
		cfg.AgoraCustomerID = req.AgoraCustomerID
		cfg.AgoraCustomerSecret = pkg.KeepSecret(req.AgoraCustomerSecret, cfg.AgoraCustomerSecret)
	// 已禁用保留：mediasoup/daily 不再通过管理端启用。
	case "mediasoup":
		cfg.MediaSoupBridgeURL = req.MediaSoupBridgeURL
		cfg.MediaSoupHost = req.MediaSoupHost
	case "srs":
		cfg.SRSHost = req.SRSHost
		cfg.SRSApiPort = req.SRSApiPort
		cfg.SRSSecret = pkg.KeepSecret(req.SRSSecret, cfg.SRSSecret)
		cfg.SRSWHIPURL = req.SRSWHIPURL
		cfg.SRSPublicHost = req.SRSPublicHost
	case "daily":
		cfg.DailyAPIKey = pkg.KeepSecret(req.DailyAPIKey, cfg.DailyAPIKey)
		cfg.DailyDomain = req.DailyDomain
	case "cloudflare":
		cfg.CFAppID = req.CFAppID
		cfg.CFAppSecret = pkg.KeepSecret(req.CFAppSecret, cfg.CFAppSecret)
		cfg.CFStunURL = req.CFStunURL
	}

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
	if frontendDisabledProviders[provider] {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "provider is temporarily disabled")
	}
	if !isValidProvider(provider) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "provider must be livekit, agora, srs, or cloudflare")
	}
	if err := s.repo.SetActiveProvider(provider); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	// 返回该 provider 的配置（若没有则用 env 默认值创建）
	return s.GetProviderConfig(provider)
}

// ListProviders 返回所有已配置 provider 的列表 + 当前激活的 provider 名称。
func (s *SFUConfigService) ListProviders() ([]model.SFUConfig, string, error) {
	active, err := s.repo.GetActiveProvider()
	if err != nil {
		return nil, "", pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	cfgs, err := s.repo.ListProviders()
	if err != nil {
		return nil, "", pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return cfgs, active, nil
}

// SyncFromEnv 启动时调用，将所有 provider 的 env 配置写入 DB（仅当该 provider 尚无 DB 记录时）。
// 不会覆盖用户在 DB 中已保存的配置。
func (s *SFUConfigService) SyncFromEnv() error {
	providers := []string{"livekit", "agora", "srs", "cloudflare"}
	for _, p := range providers {
		_, err := s.repo.GetByProvider(p)
		if err == nil {
			continue // 已有配置，不覆盖
		}
		if err != gorm.ErrRecordNotFound {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		// 无记录：用 env 默认值创建
		cfg := s.defaultConfigForProvider(p)
		if err := s.repo.Save(cfg); err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	// 设置激活 provider（env 指定或默认 livekit）
	active := s.baseCfg.SFUProvider
	if active == "" {
		active = defaultSFUProvider
	}
	if frontendDisabledProviders[active] {
		// 旧部署可能在 env 中保留已禁用 provider：不中止启动。DB 当前激活
		// 若同样不可用，则回退到默认可用 provider，避免启动后仍指向禁用后端。
		current, err := s.repo.GetActiveProvider()
		if err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		if frontendDisabledProviders[current] || !isValidProvider(current) {
			if err := s.repo.SetActiveProvider(defaultSFUProvider); err != nil {
				return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
			}
			logger.WithComponent("SFUConfig").Warnf("SFU_PROVIDER=%s is disabled and DB active=%s is unusable, falling back to %s", active, current, defaultSFUProvider)
			return nil
		}
		logger.WithComponent("SFUConfig").Warnf("SFU_PROVIDER=%s is disabled, keeping current active provider %s", active, current)
		return nil
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
	resolved.SFUProvider = cfg.Provider

	// env 的 SFU 设置直接覆盖当前激活 provider 的 DB 配置。
	// 仅当 env 显式设置了非空值时覆盖，避免用空默认值清空 DB 中已保存的值。
	switch cfg.Provider {
	case "livekit":
		if s.baseCfg.LiveKitHost == "" {
			resolved.LiveKitHost = cfg.LiveKitHost
		}
		if s.baseCfg.LiveKitKey == "" {
			resolved.LiveKitKey = cfg.LiveKitKey
		}
		if s.baseCfg.LiveKitSecret == "" {
			resolved.LiveKitSecret = cfg.LiveKitSecret
		}
	case "agora":
		if s.baseCfg.AgoraAppID == "" {
			resolved.AgoraAppID = cfg.AgoraAppID
		}
		if s.baseCfg.AgoraAppCertificate == "" {
			resolved.AgoraAppCertificate = cfg.AgoraAppCertificate
		}
		if s.baseCfg.AgoraHost == "" {
			resolved.AgoraHost = cfg.AgoraHost
		}
		if s.baseCfg.AgoraCustomerID == "" {
			resolved.AgoraCustomerID = cfg.AgoraCustomerID
		}
		if s.baseCfg.AgoraCustomerSecret == "" {
			resolved.AgoraCustomerSecret = cfg.AgoraCustomerSecret
		}
	case "mediasoup":
		if s.baseCfg.MediaSoupBridgeURL == "" {
			resolved.MediaSoupBridgeURL = cfg.MediaSoupBridgeURL
		}
		if s.baseCfg.MediaSoupHost == "" {
			resolved.MediaSoupHost = cfg.MediaSoupHost
		}
	case "srs":
		if s.baseCfg.SRSHost == "" {
			resolved.SRSHost = cfg.SRSHost
		}
		if s.baseCfg.SRSApiPort == "" {
			resolved.SRSApiPort = cfg.SRSApiPort
		}
		if s.baseCfg.SRSSecret == "" {
			resolved.SRSSecret = cfg.SRSSecret
		}
		if s.baseCfg.SRSWHIPURL == "" {
			resolved.SRSWHIPURL = cfg.SRSWHIPURL
		}
		if s.baseCfg.SRSPublicHost == "" {
			resolved.SRSPublicHost = cfg.SRSPublicHost
		}
	case "daily":
		if s.baseCfg.DailyAPIKey == "" {
			resolved.DailyAPIKey = cfg.DailyAPIKey
		}
		if s.baseCfg.DailyDomain == "" {
			resolved.DailyDomain = cfg.DailyDomain
		}
	case "cloudflare":
		if s.baseCfg.CFAppID == "" {
			resolved.CFAppID = cfg.CFAppID
		}
		if s.baseCfg.CFAppSecret == "" {
			resolved.CFAppSecret = cfg.CFAppSecret
		}
		if s.baseCfg.CFStunURL == "" || s.baseCfg.CFStunURL == "stun.cloudflare.com:3478" {
			if cfg.CFStunURL != "" {
				resolved.CFStunURL = cfg.CFStunURL
			}
		}
	}
	return &resolved, nil
}

func isValidProvider(p string) bool {
	switch p {
	case "livekit", "agora", "srs", "cloudflare":
		return true
	default:
		return false
	}
}

// defaultConfigForProvider 返回该 provider 的 env 默认配置。
func (s *SFUConfigService) defaultConfigForProvider(provider string) *model.SFUConfig {
	cfg := &model.SFUConfig{Provider: provider}
	switch provider {
	case "livekit":
		cfg.LiveKitHost = s.baseCfg.LiveKitHost
		cfg.LiveKitKey = s.baseCfg.LiveKitKey
		cfg.LiveKitSecret = s.baseCfg.LiveKitSecret
	case "agora":
		cfg.AgoraAppID = s.baseCfg.AgoraAppID
		cfg.AgoraAppCertificate = s.baseCfg.AgoraAppCertificate
		cfg.AgoraHost = s.baseCfg.AgoraHost
		cfg.AgoraCustomerID = s.baseCfg.AgoraCustomerID
		cfg.AgoraCustomerSecret = s.baseCfg.AgoraCustomerSecret
	case "mediasoup":
		cfg.MediaSoupBridgeURL = s.baseCfg.MediaSoupBridgeURL
		cfg.MediaSoupHost = s.baseCfg.MediaSoupHost
	case "srs":
		cfg.SRSHost = s.baseCfg.SRSHost
		cfg.SRSApiPort = s.baseCfg.SRSApiPort
		cfg.SRSSecret = s.baseCfg.SRSSecret
		cfg.SRSWHIPURL = s.baseCfg.SRSWHIPURL
		cfg.SRSPublicHost = s.baseCfg.SRSPublicHost
	case "daily":
		cfg.DailyAPIKey = s.baseCfg.DailyAPIKey
		cfg.DailyDomain = s.baseCfg.DailyDomain
	case "cloudflare":
		cfg.CFAppID = s.baseCfg.CFAppID
		cfg.CFAppSecret = s.baseCfg.CFAppSecret
		cfg.CFStunURL = s.baseCfg.CFStunURL
	}
	return cfg
}

// ToPublicSFUConfig 将内部配置转为管理后台安全视图。
func ToPublicSFUConfig(cfg *model.SFUConfig) *PublicSFUConfig {
	if cfg == nil {
		return nil
	}
	p := &PublicSFUConfig{
		Provider:               cfg.Provider,
		LiveKitHost:            cfg.LiveKitHost,
		LiveKitKey:             cfg.LiveKitKey,
		LiveKitSecret:          "",
		LiveKitSecretSet:       cfg.LiveKitSecret != "",
		AgoraAppID:             cfg.AgoraAppID,
		AgoraAppCertificate:    "",
		AgoraAppCertificateSet: cfg.AgoraAppCertificate != "",
		AgoraHost:              cfg.AgoraHost,
		AgoraCustomerID:        cfg.AgoraCustomerID,
		AgoraCustomerSecret:    "",
		AgoraCustomerSecretSet: cfg.AgoraCustomerSecret != "",
		MediaSoupBridgeURL:     cfg.MediaSoupBridgeURL,
		MediaSoupHost:          cfg.MediaSoupHost,
		SRSHost:                cfg.SRSHost,
		SRSApiPort:             cfg.SRSApiPort,
		SRSSecret:              "",
		SRSSecretSet:           cfg.SRSSecret != "",
		SRSWHIPURL:             cfg.SRSWHIPURL,
		SRSPublicHost:          cfg.SRSPublicHost,
		DailyAPIKey:            "",
		DailyAPIKeySet:         cfg.DailyAPIKey != "",
		DailyDomain:            cfg.DailyDomain,
		CFAppID:                cfg.CFAppID,
		CFAppSecret:            "",
		CFAppSecretSet:         cfg.CFAppSecret != "",
		CFStunURL:              cfg.CFStunURL,
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
