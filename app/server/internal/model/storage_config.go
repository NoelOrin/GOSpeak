package model

import "time"

type StorageConfig struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	ProviderType  string    `gorm:"default:'local'" json:"provider_type"` // "s3" | "local"
	Endpoint      string    `gorm:"default:''" json:"endpoint"`
	Bucket        string    `gorm:"default:''" json:"bucket"`
	Region        string    `gorm:"default:''" json:"region"`
	AccessKey     string    `gorm:"default:''" json:"-"`               // 写入加密，读取解密
	SecretKey     string    `gorm:"default:''" json:"-"`               // 写入加密，读取解密
	PublicBaseURL string    `gorm:"default:''" json:"public_base_url"` // 公开访问基础 URL（可选，用于 CDN/R2 自定义域名）
	PathPrefix    string    `gorm:"default:'uploads/'" json:"path_prefix"`
	MaxFileSize   int       `gorm:"default:5" json:"max_file_size"` // MB
	AllowedTypes  string    `gorm:"default:'image/jpeg,image/png,image/gif,image/webp'" json:"allowed_types"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (StorageConfig) TableName() string {
	return "storage_configs"
}
