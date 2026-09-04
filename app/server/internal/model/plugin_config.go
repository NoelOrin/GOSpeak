package model

import "time"

// PluginConfig 后端插件持久化配置
type PluginConfig struct {
	Name       string    `gorm:"primaryKey;size:64" json:"name"`
	Enabled    bool      `gorm:"default:true" json:"enabled"`
	ConfigJSON string    `gorm:"type:text" json:"config_json"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (PluginConfig) TableName() string {
	return "plugin_configs"
}
