package config

import (
	"os"
)

type Config struct {
	DBType      string
	DBHost      string
	DBPort      string
	DBUser      string
	DBPassword  string
	DBPath      string
	DBDSN       string
	JWTKey      string
	LiveKitHost string
	LiveKitKey  string
	LiveKitSecret string
	ServerPort  string
}

func Load() *Config {
	return &Config{
		DBType:      getEnv("DB_TYPE", "SQLite"),
		DBHost:      getEnv("DB_HOST", "localhost"),
		DBPort:      getEnv("DB_PORT", "5432"),
		DBUser:      getEnv("DB_USER", ""),
		DBPassword:  getEnv("DB_PASSWORD", ""),
		DBPath:      getEnv("DB_PATH", "app.db"),
		DBDSN:       getEnv("DB_DSN", ""),
		JWTKey:      getEnv("JWT_KEY", "default-secret"),
		LiveKitHost: getEnv("LIVEKIT_HOST", ""),
		LiveKitKey:  getEnv("LIVEKIT_KEY", ""),
		LiveKitSecret: getEnv("LIVEKIT_SECRET", ""),
		ServerPort:  getEnv("SERVER_PORT", "8098"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}