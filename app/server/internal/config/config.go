package config

import (
	"os"
)

type Config struct {
	DBType               string
	DBHost               string
	DBPort               string
	DBUser               string
	DBPassword           string
	DBPath               string
	DBDSN                string
	JWTKey               string
	SFUProvider          string
	LiveKitHost          string
	LiveKitKey           string
	LiveKitSecret        string
	AgoraAppID           string
	AgoraAppCertificate  string
	AgoraHost            string
	AgoraCustomerID      string
	AgoraCustomerSecret  string
	MediaSoupBridgeURL   string
	MediaSoupHost        string
	SRSHost              string
	SRSApiPort           string
	SRSWHIPURL           string
	SRSSecret            string
	DailyAPIKey          string
	DailyDomain          string
	CFAppID              string
	CFAppSecret          string
	CFStunURL            string
	ServerPort           string
	RedisHost            string
	RedisPort            string
	RedisPassword        string
	RedisDB              string
	EmailEnabled         bool
	SMTPHost             string
	SMTPPort             string
	SMTPUsername         string
	SMTPPassword         string
	SMTPFrom             string
	SMTPFromName         string
	EmailCodeTTL         string
	EmailSendCooldown    string
	EmailCodeSecret      string
	StorageType          string
	StorageEndpoint      string
	StorageBucket        string
	StorageRegion        string
	StorageAccessKey     string
	StorageSecretKey     string
	StoragePublicBaseURL string
	StoragePathPrefix    string
}

func Load() *Config {
	return &Config{
		DBType:               getEnv("DB_TYPE", "SQLite"),
		DBHost:               getEnv("DB_HOST", "localhost"),
		DBPort:               getEnv("DB_PORT", "5432"),
		DBUser:               getEnv("DB_USER", ""),
		DBPassword:           getEnv("DB_PASSWORD", ""),
		DBPath:               getEnv("DB_PATH", "app.db"),
		DBDSN:                getEnv("DB_DSN", ""),
		JWTKey:               getEnv("JWT_KEY", "default-secret"),
		SFUProvider:          getEnv("SFU_PROVIDER", "livekit"),
		LiveKitHost:          getEnv("LIVEKIT_HOST", ""),
		LiveKitKey:           getEnv("LIVEKIT_KEY", ""),
		LiveKitSecret:        getEnv("LIVEKIT_SECRET", ""),
		AgoraAppID:           getEnv("AGORA_APP_ID", ""),
		AgoraAppCertificate:  getEnv("AGORA_APP_CERTIFICATE", ""),
		AgoraHost:            getEnv("AGORA_HOST", ""),
		AgoraCustomerID:      getEnv("AGORA_CUSTOMER_ID", ""),
		AgoraCustomerSecret:  getEnv("AGORA_CUSTOMER_SECRET", ""),
		MediaSoupBridgeURL:   getEnv("MEDIASOUP_BRIDGE_URL", "http://localhost:3012"),
		MediaSoupHost:        getEnv("MEDIASOUP_HOST", "localhost:3012"),
		SRSHost:              getEnv("SRS_HOST", "localhost"),
		SRSApiPort:           getEnv("SRS_API_PORT", "1985"),
		SRSWHIPURL:           getEnv("SRS_WHIP_URL", "/rtc/v1/whip/"),
		SRSSecret:            getEnv("SRS_SECRET", ""),
		DailyAPIKey:          getEnv("DAILY_API_KEY", ""),
		DailyDomain:          getEnv("DAILY_DOMAIN", ""),
		CFAppID:              getEnv("CF_APP_ID", ""),
		CFAppSecret:          getEnv("CF_APP_SECRET", ""),
		CFStunURL:            getEnv("CF_STUN_URL", "stun.cloudflare.com:3478"),
		ServerPort:           getEnv("SERVER_PORT", "8098"),
		RedisHost:            getEnv("REDIS_HOST", ""),
		RedisPort:            getEnv("REDIS_PORT", "6379"),
		RedisPassword:        getEnv("REDIS_PASSWORD", ""),
		RedisDB:              getEnv("REDIS_DB", "0"),
		EmailEnabled:         getEnv("EMAIL_ENABLED", "false") == "true",
		SMTPHost:             getEnv("SMTP_HOST", ""),
		SMTPPort:             getEnv("SMTP_PORT", "587"),
		SMTPUsername:         getEnv("SMTP_USERNAME", ""),
		SMTPPassword:         getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:             getEnv("SMTP_FROM", ""),
		SMTPFromName:         getEnv("SMTP_FROM_NAME", "GoSpeak"),
		EmailCodeTTL:         getEnv("EMAIL_CODE_TTL", "10m"),
		EmailSendCooldown:    getEnv("EMAIL_SEND_COOLDOWN", "60s"),
		EmailCodeSecret:      getEnv("EMAIL_CODE_SECRET", ""),
		StorageType:          getEnv("STORAGE_TYPE", "local"),
		StorageEndpoint:      getEnv("STORAGE_ENDPOINT", ""),
		StorageBucket:        getEnv("STORAGE_BUCKET", ""),
		StorageRegion:        getEnv("STORAGE_REGION", ""),
		StorageAccessKey:     getEnv("STORAGE_ACCESS_KEY", ""),
		StorageSecretKey:     getEnv("STORAGE_SECRET_KEY", ""),
		StoragePublicBaseURL: getEnv("STORAGE_PUBLIC_BASE_URL", ""),
		StoragePathPrefix:    getEnv("STORAGE_PATH_PREFIX", "uploads/"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
