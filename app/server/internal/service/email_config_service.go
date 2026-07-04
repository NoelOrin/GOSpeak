package service

import (
	"errors"
	"strings"

	"GOSpeak/internal/config"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"gorm.io/gorm"
)

type UpdateEmailConfigRequest struct {
	Enabled           bool   `json:"enabled"`
	SMTPHost          string `json:"smtp_host"`
	SMTPPort          string `json:"smtp_port"`
	SMTPUsername      string `json:"smtp_username"`
	SMTPPassword      string `json:"smtp_password"`
	SMTPFrom          string `json:"smtp_from"`
	SMTPFromName      string `json:"smtp_from_name"`
	EmailCodeTTL      string `json:"email_code_ttl"`
	EmailSendCooldown string `json:"email_send_cooldown"`
	EmailCodeSecret   string `json:"email_code_secret"`
}

type EmailConfigService struct {
	repo    *repository.EmailConfigRepository
	baseCfg *config.Config
}

func NewEmailConfigService(repo *repository.EmailConfigRepository, baseCfg *config.Config) *EmailConfigService {
	return &EmailConfigService{repo: repo, baseCfg: baseCfg}
}

func (s *EmailConfigService) Get() (*model.EmailConfig, error) {
	cfg, err := s.repo.Get()
	if err == nil {
		return cfg, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	cfg = s.defaultConfig()
	if err := s.repo.Save(cfg); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return cfg, nil
}

func (s *EmailConfigService) UpdateFromDTO(req *UpdateEmailConfigRequest) (*model.EmailConfig, error) {
	existing, err := s.Get()
	if err != nil {
		return nil, err
	}

	cfg := &model.EmailConfig{
		ID:                1,
		Enabled:           req.Enabled,
		SMTPHost:          strings.TrimSpace(req.SMTPHost),
		SMTPPort:          strings.TrimSpace(req.SMTPPort),
		SMTPUsername:      strings.TrimSpace(req.SMTPUsername),
		SMTPPassword:      req.SMTPPassword,
		SMTPFrom:          strings.TrimSpace(req.SMTPFrom),
		SMTPFromName:      strings.TrimSpace(req.SMTPFromName),
		EmailCodeTTL:      strings.TrimSpace(req.EmailCodeTTL),
		EmailSendCooldown: strings.TrimSpace(req.EmailSendCooldown),
		EmailCodeSecret:   req.EmailCodeSecret,
	}

	if cfg.SMTPPort == "" {
		cfg.SMTPPort = existing.SMTPPort
		if cfg.SMTPPort == "" {
			cfg.SMTPPort = "587"
		}
	}
	if cfg.SMTPFromName == "" {
		cfg.SMTPFromName = existing.SMTPFromName
		if cfg.SMTPFromName == "" {
			cfg.SMTPFromName = "GoSpeak"
		}
	}
	if cfg.EmailCodeTTL == "" {
		cfg.EmailCodeTTL = existing.EmailCodeTTL
		if cfg.EmailCodeTTL == "" {
			cfg.EmailCodeTTL = "10m"
		}
	}
	if cfg.EmailSendCooldown == "" {
		cfg.EmailSendCooldown = existing.EmailSendCooldown
		if cfg.EmailSendCooldown == "" {
			cfg.EmailSendCooldown = "60s"
		}
	}
	if cfg.SMTPPassword == "" {
		cfg.SMTPPassword = existing.SMTPPassword
	}
	if cfg.EmailCodeSecret == "" {
		cfg.EmailCodeSecret = existing.EmailCodeSecret
	}

	if err := s.repo.Save(cfg); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return cfg, nil
}

func (s *EmailConfigService) ResolveConfig() (*config.Config, error) {
	saved, err := s.Get()
	if err != nil {
		return nil, err
	}
	cfg := *s.baseCfg
	cfg.EmailEnabled = saved.Enabled
	cfg.SMTPHost = saved.SMTPHost
	cfg.SMTPPort = saved.SMTPPort
	cfg.SMTPUsername = saved.SMTPUsername
	cfg.SMTPPassword = saved.SMTPPassword
	cfg.SMTPFrom = saved.SMTPFrom
	cfg.SMTPFromName = saved.SMTPFromName
	cfg.EmailCodeTTL = saved.EmailCodeTTL
	cfg.EmailSendCooldown = saved.EmailSendCooldown
	cfg.EmailCodeSecret = saved.EmailCodeSecret
	return &cfg, nil
}

func (s *EmailConfigService) IsVerificationAvailable() bool {
	cfg, err := s.ResolveConfig()
	if err != nil {
		return false
	}
	if !cfg.EmailEnabled {
		return false
	}
	return strings.TrimSpace(cfg.SMTPHost) != "" &&
		strings.TrimSpace(cfg.SMTPPort) != "" &&
		strings.TrimSpace(cfg.SMTPUsername) != "" &&
		strings.TrimSpace(cfg.SMTPPassword) != "" &&
		strings.TrimSpace(cfg.SMTPFrom) != "" &&
		strings.TrimSpace(cfg.EmailCodeSecret) != ""
}

func (s *EmailConfigService) defaultConfig() *model.EmailConfig {
	return &model.EmailConfig{
		ID:                1,
		Enabled:           s.baseCfg.EmailEnabled,
		SMTPHost:          s.baseCfg.SMTPHost,
		SMTPPort:          s.baseCfg.SMTPPort,
		SMTPUsername:      s.baseCfg.SMTPUsername,
		SMTPPassword:      s.baseCfg.SMTPPassword,
		SMTPFrom:          s.baseCfg.SMTPFrom,
		SMTPFromName:      s.baseCfg.SMTPFromName,
		EmailCodeTTL:      s.baseCfg.EmailCodeTTL,
		EmailSendCooldown: s.baseCfg.EmailSendCooldown,
		EmailCodeSecret:   s.baseCfg.EmailCodeSecret,
	}
}
