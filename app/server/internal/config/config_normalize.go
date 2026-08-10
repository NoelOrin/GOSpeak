package config

import "strings"

func (c *Config) normalize() {
	c.AppEnv = strings.TrimSpace(c.AppEnv)
	c.DBType = normalizeDBType(c.DBType)
	c.DBReadDSN = strings.TrimSpace(c.DBReadDSN)
	c.DBReadHost = strings.TrimSpace(c.DBReadHost)
	c.DBReadPort = strings.TrimSpace(c.DBReadPort)
	c.DBReadUser = strings.TrimSpace(c.DBReadUser)
	c.DBReadPassword = strings.TrimSpace(c.DBReadPassword)
	c.DBReadDBName = strings.TrimSpace(c.DBReadDBName)
	c.DBReplicaLagThreshold = strings.TrimSpace(c.DBReplicaLagThreshold)
	c.SFUProvider = strings.ToLower(strings.TrimSpace(c.SFUProvider))
	c.StateStore = strings.ToLower(strings.TrimSpace(c.StateStore))
	c.StorageType = strings.ToLower(strings.TrimSpace(c.StorageType))
	c.ServerPort = strings.TrimSpace(c.ServerPort)
	c.CORSOrigin = strings.TrimSpace(c.CORSOrigin)
	if c.CORSOrigin == "" {
		c.CORSOrigin = "*"
	}
	c.WSAllowedOrigins = strings.TrimSpace(c.WSAllowedOrigins)
	c.ClusterRole = strings.ToLower(strings.TrimSpace(c.ClusterRole))
	c.AccessCookieName = strings.TrimSpace(c.AccessCookieName)
	c.RefreshCookieName = strings.TrimSpace(c.RefreshCookieName)
	c.CookieDomain = strings.TrimSpace(c.CookieDomain)
	c.CookiePath = strings.TrimSpace(c.CookiePath)
	c.CookieSecure = strings.ToLower(strings.TrimSpace(c.CookieSecure))
	c.CookieSameSite = strings.ToLower(strings.TrimSpace(c.CookieSameSite))
	c.ClusterNodeID = strings.TrimSpace(c.ClusterNodeID)
	c.ClusterAdvertiseURL = strings.TrimSpace(c.ClusterAdvertiseURL)
	c.ClusterAgentURL = strings.TrimSpace(c.ClusterAgentURL)
	c.ClusterAgentToken = strings.TrimSpace(c.ClusterAgentToken)
	c.ClusterNodeSecret = strings.TrimSpace(c.ClusterNodeSecret)
	c.ClusterHeartbeatInterval = strings.TrimSpace(c.ClusterHeartbeatInterval)
	c.ClusterHeartbeatTimeout = strings.TrimSpace(c.ClusterHeartbeatTimeout)
	c.ClusterLabels = strings.TrimSpace(c.ClusterLabels)
	c.ClusterEntryURL = strings.TrimSpace(c.ClusterEntryURL)
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
	if c.DBReadPort == "" {
		c.DBReadPort = c.DBPort
	}
	if c.DBUser == "" {
		switch c.DBType {
		case "PostgreSQL":
			c.DBUser = "postgres"
		case "MySQL":
			c.DBUser = "root"
		}
	}
	if c.DBReadUser == "" {
		c.DBReadUser = c.DBUser
	}
	if c.DBReadPassword == "" {
		c.DBReadPassword = c.DBPassword
	}
	if c.DBReadHost == "" {
		c.DBReadHost = c.DBHost
	}
	if c.RedisPort == "" {
		c.RedisPort = "6379"
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
	if c.StateStore == "" {
		c.StateStore = "auto"
	}
	if c.SFUProvider == "" {
		c.SFUProvider = "livekit"
	}
	if c.ServerPort == "" {
		c.ServerPort = "8998"
	}
	if c.ClusterRole == "" {
		c.ClusterRole = "all"
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
	if c.ClusterHeartbeatInterval == "" {
		c.ClusterHeartbeatInterval = "5s"
	}
	if c.ClusterHeartbeatTimeout == "" {
		c.ClusterHeartbeatTimeout = "30s"
	}
	if c.DBReplicaLagThreshold == "" {
		c.DBReplicaLagThreshold = "5s"
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
