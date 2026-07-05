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
	SRSWHIPPort         string `json:"srs_whip_port"`
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

func (s *SFUConfigService) Get() (*model.SFUConfig, error) {
	cfg, err := s.repo.Get()
	if err == nil {
		return cfg, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	cfg = s.defaultConfig()
	if err := s.repo.Save(cfg); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return cfg, nil
}

func (s *SFUConfigService) Update(cfg *model.SFUConfig) (*model.SFUConfig, error) {
	if cfg.Provider != "livekit" && cfg.Provider != "agora" && cfg.Provider != "mediasoup" && cfg.Provider != "srs" && cfg.Provider != "daily" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "provider must be livekit, agora, mediasoup, srs, or daily")
	}
	cfg.ID = 1
	if err := s.repo.Save(cfg); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return cfg, nil
}

// SyncFromEnv 在每次初始化时把当前 env 覆盖写入 DB (singleton ID=1)。
// env 是唯一真相源: 启动后 DB 行 = env 值, API Update 的改动重启后会被 env 覆盖。
func (s *SFUConfigService) SyncFromEnv() error {
	cfg := s.defaultConfig()
	if err := s.repo.Save(cfg); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// UpdateFromDTO updates SFU config from a handler-level DTO.
func (s *SFUConfigService) UpdateFromDTO(req *UpdateSFUConfigRequest) (*model.SFUConfig, error) {
	if req.Provider != "livekit" && req.Provider != "agora" && req.Provider != "mediasoup" && req.Provider != "srs" && req.Provider != "daily" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "provider must be livekit, agora, mediasoup, srs, or daily")
	}
	cfg := &model.SFUConfig{
		ID:                  1,
		Provider:            req.Provider,
		LiveKitHost:         req.LiveKitHost,
		LiveKitKey:          req.LiveKitKey,
		LiveKitSecret:       req.LiveKitSecret,
		AgoraAppID:          req.AgoraAppID,
		AgoraAppCertificate: req.AgoraAppCertificate,
		AgoraHost:           req.AgoraHost,
		AgoraCustomerID:     req.AgoraCustomerID,
		AgoraCustomerSecret: req.AgoraCustomerSecret,
		MediaSoupBridgeURL:  req.MediaSoupBridgeURL,
		MediaSoupHost:       req.MediaSoupHost,
		SRSHost:             req.SRSHost,
		SRSApiPort:          req.SRSApiPort,
		SRSWHIPPort:         req.SRSWHIPPort,
		SRSSecret:           req.SRSSecret,
		DailyAPIKey:         req.DailyAPIKey,
		DailyDomain:         req.DailyDomain,
	}
	if err := s.repo.Save(cfg); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return cfg, nil
}

func (s *SFUConfigService) ResolveConfig() (*config.Config, error) {
	saved, err := s.Get()
	if err != nil {
		return nil, err
	}
	cfg := *s.baseCfg
	cfg.SFUProvider = saved.Provider
	cfg.LiveKitHost = saved.LiveKitHost
	cfg.LiveKitKey = saved.LiveKitKey
	cfg.LiveKitSecret = saved.LiveKitSecret
	cfg.AgoraAppID = saved.AgoraAppID
	cfg.AgoraAppCertificate = saved.AgoraAppCertificate
	cfg.AgoraHost = saved.AgoraHost
	cfg.AgoraCustomerID = saved.AgoraCustomerID
	cfg.AgoraCustomerSecret = saved.AgoraCustomerSecret
	cfg.MediaSoupBridgeURL = saved.MediaSoupBridgeURL
	cfg.MediaSoupHost = saved.MediaSoupHost
	cfg.SRSHost = saved.SRSHost
	cfg.SRSApiPort = saved.SRSApiPort
	cfg.SRSWHIPPort = saved.SRSWHIPPort
	cfg.SRSSecret = saved.SRSSecret
	cfg.DailyAPIKey = saved.DailyAPIKey
	cfg.DailyDomain = saved.DailyDomain
	return &cfg, nil
}

func (s *SFUConfigService) defaultConfig() *model.SFUConfig {
	provider := s.baseCfg.SFUProvider
	if provider == "" {
		provider = "livekit"
	}
	return &model.SFUConfig{
		ID:                  1,
		Provider:            provider,
		LiveKitHost:         s.baseCfg.LiveKitHost,
		LiveKitKey:          s.baseCfg.LiveKitKey,
		LiveKitSecret:       s.baseCfg.LiveKitSecret,
		AgoraAppID:          s.baseCfg.AgoraAppID,
		AgoraAppCertificate: s.baseCfg.AgoraAppCertificate,
		AgoraHost:           s.baseCfg.AgoraHost,
		AgoraCustomerID:     s.baseCfg.AgoraCustomerID,
		AgoraCustomerSecret: s.baseCfg.AgoraCustomerSecret,
		MediaSoupBridgeURL:  s.baseCfg.MediaSoupBridgeURL,
		MediaSoupHost:       s.baseCfg.MediaSoupHost,
		SRSHost:             s.baseCfg.SRSHost,
		SRSApiPort:          s.baseCfg.SRSApiPort,
		SRSWHIPPort:         s.baseCfg.SRSWHIPPort,
		SRSSecret:           s.baseCfg.SRSSecret,
		DailyAPIKey:         s.baseCfg.DailyAPIKey,
		DailyDomain:         s.baseCfg.DailyDomain,
	}
}
