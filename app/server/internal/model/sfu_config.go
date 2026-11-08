package model

import "time"

// SFUConfig 保存 LiveKit 的 SFU 配置。
type SFUConfig struct {
	Provider      string    `gorm:"primaryKey;size:32;not null;default:livekit" json:"provider"`
	LiveKitHost   string    `gorm:"size:255" json:"livekit_host"`
	LiveKitKey    string    `gorm:"size:255" json:"livekit_key"`
	LiveKitSecret string    `gorm:"size:255" json:"-"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (SFUConfig) TableName() string {
	return "sfu_configs"
}

// SFUActiveProvider 记录当前激活的 SFU provider。
// 与 SFUConfig 分离，切换 provider 时不会覆盖配置数据。
type SFUActiveProvider struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Provider string `gorm:"size:32;not null" json:"provider"`
}

func (SFUActiveProvider) TableName() string {
	return "sfu_active_provider"
}
