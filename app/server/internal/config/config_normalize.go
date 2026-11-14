package config

import "strings"

func (c *Config) normalize() {
	c.AppEnv = strings.TrimSpace(c.AppEnv)
	c.DBType = normalizeDBType(c.DBType)
	c.SFUProvider = strings.ToLower(strings.TrimSpace(c.SFUProvider))
	c.LiveKitHost = strings.TrimSpace(c.LiveKitHost)
	c.LiveKitPublicURL = normalizeLiveKitPublicURL(c.LiveKitPublicURL)
	c.StorageType = strings.ToLower(strings.TrimSpace(c.StorageType))
	c.ServerPort = strings.TrimSpace(c.ServerPort)
	c.CORSOrigin = strings.TrimSpace(c.CORSOrigin)
	if c.CORSOrigin == "" {
		c.CORSOrigin = "*"
	}
	c.WSAllowedOrigins = strings.TrimSpace(c.WSAllowedOrigins)
	c.AccessCookieName = strings.TrimSpace(c.AccessCookieName)
	c.RefreshCookieName = strings.TrimSpace(c.RefreshCookieName)
	c.CookieDomain = strings.TrimSpace(c.CookieDomain)
	c.CookiePath = strings.TrimSpace(c.CookiePath)
	c.CookieSecure = strings.ToLower(strings.TrimSpace(c.CookieSecure))
	c.CookieSameSite = strings.ToLower(strings.TrimSpace(c.CookieSameSite))
	c.MetricsToken = strings.TrimSpace(c.MetricsToken)
	if c.DBPath == "" {
		c.DBPath = "db/app.db"
	}
	if c.DBPort == "" {
		switch c.DBType {
		case "PostgreSQL":
			c.DBPort = "5432"
		case "MySQL":
			c.DBPort = "3306"
		}
	}
	if c.DBUser == "" {
		switch c.DBType {
		case "PostgreSQL":
			c.DBUser = "postgres"
		case "MySQL":
			c.DBUser = "root"
		}
	}
	if c.JWTKeyTTL == "" {
		c.JWTKeyTTL = "24h"
	}
	if c.NATSConnectTimeout == "" {
		c.NATSConnectTimeout = "2s"
	}
	if c.NATSSubjectPrefix == "" {
		c.NATSSubjectPrefix = "gospeak"
	}
	if c.SFUProvider == "" {
		c.SFUProvider = DefaultSFUProvider
	}
	if c.ServerPort == "" {
		c.ServerPort = "8998"
	}
	if c.AccessCookieName == "" {
		c.AccessCookieName = "gospeak_token"
	}
	if c.RefreshCookieName == "" {
		c.RefreshCookieName = "gospeak_refresh_token"
	}
	if c.CookiePath == "" {
		c.CookiePath = "/"
	}
	if c.CookieSecure == "" {
		c.CookieSecure = "auto"
	}
	if c.CookieSameSite == "" {
		c.CookieSameSite = "lax"
	}

	c.LogLevel = strings.ToLower(strings.TrimSpace(c.LogLevel))
	c.LogFormat = strings.ToLower(strings.TrimSpace(c.LogFormat))
	c.LogOutput = strings.ToLower(strings.TrimSpace(c.LogOutput))
	c.LogFile = strings.TrimSpace(c.LogFile)
}

func normalizeDBType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "sqlite":
		return "SQLite"
	case "postgres", "postgresql", "postgressql":
		// 兼容历史拼写 PostgresSQL
		return "PostgreSQL"
	case "mysql":
		return "MySQL"
	default:
		return strings.TrimSpace(raw)
	}
}

// normalizeLiveKitPublicURL 把浏览器侧地址归一为"显式地址或空"。
// 空 = 同源自动推导（应用内置 /livekit 反代），不再回退到 LIVEKIT_HOST：
// 容器内网服务名（ws://livekit:7880）浏览器无法解析，回退过去只会造成连接失败。
func normalizeLiveKitPublicURL(raw string) string {
	value := strings.TrimSpace(raw)
	switch strings.ToLower(value) {
	case "", "auto", "same-origin", "same_origin":
		return ""
	}
	return value
}
