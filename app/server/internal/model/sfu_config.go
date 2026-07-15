package model

import "time"

// SFUConfig 每个 SFU provider 一行，以 provider 为主键。
// 切换 provider 时互不覆盖，每个 provider 的配置数据独立持久化。
type SFUConfig struct {
	Provider            string    `gorm:"primaryKey;size:32;not null;default:livekit" json:"provider"`
	LiveKitHost         string    `gorm:"size:255" json:"livekit_host"`
	LiveKitKey          string    `gorm:"size:255" json:"livekit_key"`
	LiveKitSecret       string    `gorm:"size:255" json:"-"`
	AgoraAppID          string    `gorm:"size:255" json:"agora_app_id"`
	AgoraAppCertificate string    `gorm:"size:255" json:"-"`
	AgoraHost           string    `gorm:"size:255" json:"agora_host"`
	AgoraCustomerID     string    `gorm:"size:255" json:"agora_customer_id"`
	AgoraCustomerSecret string    `gorm:"size:255" json:"-"`
	MediaSoupBridgeURL  string    `gorm:"size:255" json:"mediasoup_bridge_url"`
	MediaSoupHost       string    `gorm:"size:255" json:"mediasoup_host"`
	SRSHost             string    `gorm:"size:255" json:"srs_host"`
	SRSApiPort          string    `gorm:"size:255;default:1985" json:"srs_api_port"`
	SRSSecret           string    `gorm:"size:255" json:"-"`
	SRSWHIPURL          string    `gorm:"size:255" json:"srs_whip_url"`
	SRSPublicHost       string    `gorm:"size:255" json:"srs_public_host"`
	DailyAPIKey         string    `gorm:"size:255" json:"-"`
	DailyDomain         string    `gorm:"size:255" json:"daily_domain"`
	CFAppID             string    `gorm:"size:255" json:"cf_app_id"`
	CFAppSecret         string    `gorm:"size:255" json:"-"`
	CFStunURL           string    `gorm:"size:255;default:stun.cloudflare.com:3478" json:"cf_stun_url"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
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
