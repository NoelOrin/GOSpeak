package service

import (
	"GOSpeak/internal/config"
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
	DailyAPIKey         string `json:"daily_api_key"`
	DailyDomain         string `json:"daily_domain"`
}

type SFUConfigService struct {
	repo    *repository.SFUConfigRepository
	baseCfg *config.Config
}

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
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "invalid provider: must be livekit, agora, mediasoup, srs, or daily")
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
	if !isValidProvider(req.Provider) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "provider must be livekit, agora, mediasoup, srs, or daily")
	}

	// 读取现有配置（若有），保留未在请求中发送的字段
	existing, err := s.repo.GetByProvider(req.Provider)
	cfg := &model.SFUConfig{Provider: req.Provider}
	if err == nil {
		cfg = existing
	}

	// 应用所有请求字段。每个 provider 行独立，行内全量字段写入。
	cfg.LiveKitHost = req.LiveKitHost
	cfg.LiveKitKey = req.LiveKitKey
	cfg.LiveKitSecret = req.LiveKitSecret
	cfg.AgoraAppID = req.AgoraAppID
	cfg.AgoraAppCertificate = req.AgoraAppCertificate
	cfg.AgoraHost = req.AgoraHost
	cfg.AgoraCustomerID = req.AgoraCustomerID
	cfg.AgoraCustomerSecret = req.AgoraCustomerSecret
	cfg.MediaSoupBridgeURL = req.MediaSoupBridgeURL
	cfg.MediaSoupHost = req.MediaSoupHost
	cfg.SRSHost = req.SRSHost
	cfg.SRSApiPort = req.SRSApiPort
	cfg.SRSSecret = req.SRSSecret
	cfg.DailyAPIKey = req.DailyAPIKey
	cfg.DailyDomain = req.DailyDomain

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
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "provider must be livekit, agora, mediasoup, srs, or daily")
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
	providers := []string{"livekit", "agora", "mediasoup", "srs", "daily"}
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
		active = "livekit"
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
	resolved.LiveKitHost = cfg.LiveKitHost
	resolved.LiveKitKey = cfg.LiveKitKey
	resolved.LiveKitSecret = cfg.LiveKitSecret
	resolved.AgoraAppID = cfg.AgoraAppID
	resolved.AgoraAppCertificate = cfg.AgoraAppCertificate
	resolved.AgoraHost = cfg.AgoraHost
	resolved.AgoraCustomerID = cfg.AgoraCustomerID
	resolved.AgoraCustomerSecret = cfg.AgoraCustomerSecret
	resolved.MediaSoupBridgeURL = cfg.MediaSoupBridgeURL
	resolved.MediaSoupHost = cfg.MediaSoupHost
	resolved.SRSHost = cfg.SRSHost
	resolved.SRSApiPort = cfg.SRSApiPort
	resolved.SRSSecret = cfg.SRSSecret
	resolved.DailyAPIKey = cfg.DailyAPIKey
	resolved.DailyDomain = cfg.DailyDomain
	return &resolved, nil
}

func isValidProvider(p string) bool {
	switch p {
	case "livekit", "agora", "mediasoup", "srs", "daily":
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
	case "daily":
		cfg.DailyAPIKey = s.baseCfg.DailyAPIKey
		cfg.DailyDomain = s.baseCfg.DailyDomain
	}
	return cfg
}
