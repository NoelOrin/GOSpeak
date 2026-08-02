package service

import (
	"GOSpeak/internal/config"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/storage"
	"errors"
	"io"
	"sync"

	"gorm.io/gorm"
)

// UpdateStorageConfigRequest is the handler-facing DTO for updating storage configuration.
type UpdateStorageConfigRequest struct {
	ProviderType  string `json:"provider_type"`
	Endpoint      string `json:"endpoint"`
	Bucket        string `json:"bucket"`
	Region        string `json:"region"`
	AccessKey     string `json:"access_key"`
	SecretKey     string `json:"secret_key"`
	PublicBaseURL string `json:"public_base_url"`
	PathPrefix    string `json:"path_prefix"`
	MaxFileSize   int    `json:"max_file_size"`
	AllowedTypes  string `json:"allowed_types"`
}

// PublicStorageConfig 管理后台存储配置视图：密钥不回显明文。
type PublicStorageConfig struct {
	ProviderType  string `json:"provider_type"`
	Endpoint      string `json:"endpoint"`
	Bucket        string `json:"bucket"`
	Region        string `json:"region"`
	AccessKey     string `json:"access_key"`
	AccessKeySet  bool   `json:"access_key_set"`
	SecretKey     string `json:"secret_key"`
	SecretKeySet  bool   `json:"secret_key_set"`
	PublicBaseURL string `json:"public_base_url"`
	PathPrefix    string `json:"path_prefix"`
	MaxFileSize   int    `json:"max_file_size"`
	AllowedTypes  string `json:"allowed_types"`
}

// StorageService 存储服务，管理存储配置和 provider 生命周期
type StorageService struct {
	repo     *repository.StorageConfigRepository
	cfg      *config.Config
	provider storage.Provider
	mu       sync.RWMutex
}

// NewStorageService 创建存储服务
func NewStorageService(repo *repository.StorageConfigRepository, cfg *config.Config) *StorageService {
	return &StorageService{repo: repo, cfg: cfg}
}

// GetProvider 获取当前活跃的存储 provider（懒加载）
func (s *StorageService) GetProvider() (storage.Provider, error) {
	s.mu.RLock()
	if s.provider != nil {
		p := s.provider
		s.mu.RUnlock()
		return p, nil
	}
	s.mu.RUnlock()

	return s.ReloadProvider()
}

// GetConfig 获取存储配置
func (s *StorageService) GetConfig() (*model.StorageConfig, error) {
	cfg, err := s.repo.GetConfig()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 数据库无记录，返回环境变量 fallback 配置
			return s.getEnvFallbackConfig(), nil
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return cfg, nil
}

// UpdateConfig 更新存储配置并重建 provider
func (s *StorageService) UpdateConfig(cfg model.StorageConfig) error {
	// 如果 access_key/secret_key 为空，保留原值
	existing, err := s.repo.GetConfig()
	if err == nil && existing != nil {
		cfg.AccessKey = pkg.KeepSecret(cfg.AccessKey, existing.AccessKey)
		cfg.SecretKey = pkg.KeepSecret(cfg.SecretKey, existing.SecretKey)
	}

	if err := s.repo.SaveConfig(&cfg); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	if _, err := s.ReloadProvider(); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "config saved but provider reload failed: "+err.Error())
	}
	return nil
}

// UpdateConfigFromDTO updates storage configuration from a handler-level DTO.
func (s *StorageService) UpdateConfigFromDTO(req UpdateStorageConfigRequest) (*PublicStorageConfig, error) {
	cfg := model.StorageConfig{
		ProviderType:  req.ProviderType,
		Endpoint:      req.Endpoint,
		Bucket:        req.Bucket,
		Region:        req.Region,
		AccessKey:     req.AccessKey,
		SecretKey:     req.SecretKey,
		PublicBaseURL: req.PublicBaseURL,
		PathPrefix:    req.PathPrefix,
		MaxFileSize:   req.MaxFileSize,
		AllowedTypes:  req.AllowedTypes,
	}
	if err := s.UpdateConfig(cfg); err != nil {
		return nil, err
	}
	saved, err := s.GetConfig()
	if err != nil {
		return nil, err
	}
	return ToPublicStorageConfig(saved), nil
}

// TestConnectionFromDTO 使用表单里的配置临时创建 provider 并测试连接，不写入数据库。
func (s *StorageService) TestConnectionFromDTO(req UpdateStorageConfigRequest) error {
	cfg := model.StorageConfig{
		ProviderType:  req.ProviderType,
		Endpoint:      req.Endpoint,
		Bucket:        req.Bucket,
		Region:        req.Region,
		AccessKey:     req.AccessKey,
		SecretKey:     req.SecretKey,
		PublicBaseURL: req.PublicBaseURL,
		PathPrefix:    req.PathPrefix,
		MaxFileSize:   req.MaxFileSize,
		AllowedTypes:  req.AllowedTypes,
	}

	existing, err := s.repo.GetConfig()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if existing != nil {
		cfg.AccessKey = pkg.KeepSecret(cfg.AccessKey, existing.AccessKey)
		cfg.SecretKey = pkg.KeepSecret(cfg.SecretKey, existing.SecretKey)
	}

	var p storage.Provider
	switch cfg.ProviderType {
	case "s3":
		p, err = storage.NewS3Provider(cfg)
	case "local":
		p = storage.NewLocalProvider("", "", cfg.PublicBaseURL)
	default:
		return pkg.NewAppError(pkg.INVALID_PARAMS, "provider_type must be s3 or local")
	}
	if err != nil {
		return pkg.NewAppError(pkg.STORAGE_ERROR, "init s3 provider failed: "+err.Error())
	}
	if err := p.TestConnection(); err != nil {
		return pkg.NewAppError(pkg.STORAGE_ERROR, err.Error())
	}
	return nil
}

// ReloadProvider 重建存储 provider
func (s *StorageService) ReloadProvider() (storage.Provider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.provider != nil {
		return s.provider, nil
	}

	cfg, err := s.getConfigForProvider()
	if err != nil {
		return nil, pkg.NewAppError(pkg.STORAGE_NOT_CONFIGURED, err.Error())
	}

	var p storage.Provider
	if cfg.ProviderType == "s3" {
		p, err = storage.NewS3Provider(*cfg)
		if err != nil {
			return nil, pkg.NewAppError(pkg.STORAGE_ERROR, "init s3 provider failed: "+err.Error())
		}
	} else {
		p = storage.NewLocalProvider("", "", cfg.PublicBaseURL)
	}

	s.provider = p
	return p, nil
}

// PresignUpload 获取预签名上传 URL
func (s *StorageService) PresignUpload(key, contentType string, maxSize int64) (*storage.PresignedResult, error) {
	p, err := s.GetProvider()
	if err != nil {
		return nil, err
	}
	return p.PresignUpload(key, contentType, maxSize)
}

// UploadFromReader 从 reader 上传文件
func (s *StorageService) UploadFromReader(key string, reader io.Reader, size int64, contentType string) (string, error) {
	p, err := s.GetProvider()
	if err != nil {
		return "", err
	}
	return p.UploadFromReader(key, reader, size, contentType)
}

// DeleteObject 删除存储对象
func (s *StorageService) DeleteObject(key string) error {
	p, err := s.GetProvider()
	if err != nil {
		return err
	}
	return p.Delete(key)
}

// getConfigForProvider 获取用于初始化 provider 的配置（数据库 → 环境变量 → 默认）
func (s *StorageService) getConfigForProvider() (*model.StorageConfig, error) {
	cfg, err := s.repo.GetConfig()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s.getEnvFallbackConfig(), nil
		}
		return nil, err
	}
	return cfg, nil
}

// getEnvFallbackConfig builds a fallback config from the injected Config struct.
func (s *StorageService) getEnvFallbackConfig() *model.StorageConfig {
	return &model.StorageConfig{
		ProviderType:  s.cfg.StorageType,
		Endpoint:      s.cfg.StorageEndpoint,
		Bucket:        s.cfg.StorageBucket,
		Region:        s.cfg.StorageRegion,
		AccessKey:     s.cfg.StorageAccessKey,
		SecretKey:     s.cfg.StorageSecretKey,
		PublicBaseURL: s.cfg.StoragePublicBaseURL,
		PathPrefix:    s.cfg.StoragePathPrefix,
		MaxFileSize:   5,
		AllowedTypes:  "image/jpeg,image/png,image/gif,image/webp,application/pdf,text/plain",
	}
}

// ToPublicStorageConfig 转为管理后台安全视图。
func ToPublicStorageConfig(cfg *model.StorageConfig) *PublicStorageConfig {
	if cfg == nil {
		return nil
	}
	return &PublicStorageConfig{
		ProviderType:  cfg.ProviderType,
		Endpoint:      cfg.Endpoint,
		Bucket:        cfg.Bucket,
		Region:        cfg.Region,
		AccessKey:     "",
		AccessKeySet:  cfg.AccessKey != "",
		SecretKey:     "",
		SecretKeySet:  cfg.SecretKey != "",
		PublicBaseURL: cfg.PublicBaseURL,
		PathPrefix:    cfg.PathPrefix,
		MaxFileSize:   cfg.MaxFileSize,
		AllowedTypes:  cfg.AllowedTypes,
	}
}
