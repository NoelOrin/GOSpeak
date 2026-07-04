package model

import "time"

type SFUConfig struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	Provider            string    `gorm:"size:32;not null;default:livekit" json:"provider"`
	LiveKitHost         string    `gorm:"size:255" json:"livekit_host"`
	LiveKitKey          string    `gorm:"size:255" json:"livekit_key"`
	LiveKitSecret       string    `gorm:"size:255" json:"livekit_secret"`
	AgoraAppID          string    `gorm:"size:255" json:"agora_app_id"`
	AgoraAppCertificate string    `gorm:"size:255" json:"agora_app_certificate"`
	AgoraHost           string    `gorm:"size:255" json:"agora_host"`
	AgoraCustomerID     string    `gorm:"size:255" json:"agora_customer_id"`
	AgoraCustomerSecret string    `gorm:"size:255" json:"agora_customer_secret"`
	MediaSoupBridgeURL  string    `gorm:"size:255" json:"mediasoup_bridge_url"`
	MediaSoupHost       string    `gorm:"size:255" json:"mediasoup_host"`
	SRSHost             string    `gorm:"size:255" json:"srs_host"`
	SRSApiPort          string    `gorm:"size:255;default:1985" json:"srs_api_port"`
	SRSWHIPPort         string    `gorm:"size:255;default:1985" json:"srs_whip_port"`
	SRSSecret           string    `gorm:"size:255" json:"srs_secret"`
	DailyAPIKey         string    `gorm:"size:255" json:"daily_api_key"`
	DailyDomain         string    `gorm:"size:255" json:"daily_domain"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (SFUConfig) TableName() string {
	return "sfu_configs"
}
