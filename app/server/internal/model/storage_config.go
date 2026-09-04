package model

import "time"

type StorageConfig struct {
	ID            uint   `gorm:"primaryKey" json:"id"`
	ProviderType  string `json:"provider_type"` // "s3" | "local"
	Endpoint      string `json:"endpoint"`
	Bucket        string `json:"bucket"`
	Region        string `json:"region"`
	AccessKey     string `json:"-"`               // 写入加密，读取解密
	SecretKey     string `json:"-"`               // 写入加密，读取解密
	PublicBaseURL string `json:"public_base_url"` // 公开访问基础 URL（可选，用于 CDN/R2 自定义域名）
	// PathPrefix / AllowedTypes / ProviderType 不用 GORM default：
	// 1) 逗号/斜杠会被 tag 解析器截断，生成非法 AutoMigrate SQL
	// 2) 含特殊字符的 default 也会让 glebarez migrator 解析现有 DDL 失败
	// 默认值由 service 层填入（storage_service.GetConfig / seed）。
	PathPrefix   string    `json:"path_prefix"`
	MaxFileSize  int       `json:"max_file_size"` // MB
	AllowedTypes string    `json:"allowed_types"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (StorageConfig) TableName() string {
	return "storage_configs"
}
