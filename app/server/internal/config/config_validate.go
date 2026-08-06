package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Validate 做启动期校验，尽早暴露配置错误。
func (c *Config) Validate() error {
	var errs []string

	switch c.DBType {
	case "SQLite", "PostgreSQL", "MySQL":
	default:
		errs = append(errs, fmt.Sprintf("DB_TYPE %q unsupported (SQLite|PostgreSQL|MySQL)", c.DBType))
	}

	if c.DBType != "SQLite" && c.DBDSN == "" {
		if strings.TrimSpace(c.DBHost) == "" {
			errs = append(errs, "DB_HOST is required when DB_TYPE is not SQLite and DB_DSN is empty")
		}
	}

	switch c.SFUProvider {
	case "livekit", "srs", "agora", "cloudflare":
	default:
		errs = append(errs, fmt.Sprintf("SFU_PROVIDER %q unsupported", c.SFUProvider))
	}

	switch c.StateStore {
	case "auto", "redis", "nats", "none":
	default:
		errs = append(errs, fmt.Sprintf("STATE_STORE %q unsupported (auto|redis|nats|none)", c.StateStore))
	}

	switch c.ClusterRole {
	case "agent", "worker", "all":
	default:
		errs = append(errs, fmt.Sprintf("GOSPEAK_ROLE %q unsupported (agent|worker|all)", c.ClusterRole))
	}
	if c.ClusterRole == "worker" && strings.TrimSpace(c.ClusterAgentURL) == "" {
		errs = append(errs, "CLUSTER_AGENT_URL is required when GOSPEAK_ROLE=worker")
	}
	if c.ClusterRole == "worker" && strings.TrimSpace(c.ClusterAgentToken) == "" {
		errs = append(errs, "CLUSTER_AGENT_TOKEN is required when GOSPEAK_ROLE=worker")
	}
	if c.ClusterRole == "worker" && strings.TrimSpace(c.ClusterAdvertiseURL) == "" {
		errs = append(errs, "CLUSTER_ADVERTISE_URL is required when GOSPEAK_ROLE=worker")
	}
	if c.ClusterMaxServers < 1 {
		errs = append(errs, "CLUSTER_MAX_SERVERS must be >= 1")
	}
	if c.ClusterMaxRooms < 1 {
		errs = append(errs, "CLUSTER_MAX_ROOMS must be >= 1")
	}

	switch c.StorageType {
	case "local", "s3":
	default:
		errs = append(errs, fmt.Sprintf("STORAGE_TYPE %q unsupported (local|s3)", c.StorageType))
	}

	if port, err := strconv.Atoi(c.ServerPort); err != nil || port < 1 || port > 65535 {
		errs = append(errs, fmt.Sprintf("SERVER_PORT %q invalid", c.ServerPort))
	}

	if c.LogLevel != "" {
		switch c.LogLevel {
		case "trace", "debug", "info", "warn", "warning", "error", "fatal", "panic":
		default:
			errs = append(errs, fmt.Sprintf("LOG_LEVEL %q unsupported", c.LogLevel))
		}
	}
	if c.LogFormat != "" {
		switch c.LogFormat {
		case "text", "json":
		default:
			errs = append(errs, fmt.Sprintf("LOG_FORMAT %q unsupported (text|json)", c.LogFormat))
		}
	}
	if c.LogOutput != "" {
		switch c.LogOutput {
		case "stdout", "stderr", "file", "both":
		default:
			errs = append(errs, fmt.Sprintf("LOG_OUTPUT %q unsupported (stdout|stderr|file|both)", c.LogOutput))
		}
	}

	if _, err := time.ParseDuration(strings.TrimSpace(c.JWTKeyTTL)); err != nil {
		errs = append(errs, fmt.Sprintf("JWT_KEY_TTL %q invalid duration", c.JWTKeyTTL))
	}
	if _, err := time.ParseDuration(strings.TrimSpace(c.NATSConnectTimeout)); err != nil {
		errs = append(errs, fmt.Sprintf("NATS_CONNECT_TIMEOUT %q invalid duration", c.NATSConnectTimeout))
	}
	if _, err := time.ParseDuration(strings.TrimSpace(c.ClusterHeartbeatInterval)); err != nil {
		errs = append(errs, fmt.Sprintf("CLUSTER_HEARTBEAT_INTERVAL %q invalid duration", c.ClusterHeartbeatInterval))
	}
	if _, err := time.ParseDuration(strings.TrimSpace(c.ClusterHeartbeatTimeout)); err != nil {
		errs = append(errs, fmt.Sprintf("CLUSTER_HEARTBEAT_TIMEOUT %q invalid duration", c.ClusterHeartbeatTimeout))
	}
	if _, err := time.ParseDuration(strings.TrimSpace(c.EmailCodeTTL)); err != nil {
		errs = append(errs, fmt.Sprintf("EMAIL_CODE_TTL %q invalid duration", c.EmailCodeTTL))
	}
	if _, err := time.ParseDuration(strings.TrimSpace(c.EmailSendCooldown)); err != nil {
		errs = append(errs, fmt.Sprintf("EMAIL_SEND_COOLDOWN %q invalid duration", c.EmailSendCooldown))
	}

	if c.NATSEmbeddedPort != "" {
		if p, err := strconv.Atoi(c.NATSEmbeddedPort); err != nil || p < 0 || p > 65535 {
			errs = append(errs, fmt.Sprintf("NATS_EMBEDDED_PORT %q invalid", c.NATSEmbeddedPort))
		}
	}

	if _, err := strconv.Atoi(strings.TrimSpace(c.RedisDB)); err != nil {
		errs = append(errs, fmt.Sprintf("REDIS_DB %q invalid", c.RedisDB))
	}

	if c.EmailEnabled && strings.TrimSpace(c.EmailCodeSecret) == "" {
		errs = append(errs, "EMAIL_CODE_SECRET is required when EMAIL_ENABLED=true")
	}

	// 生产环境强制要求加密密钥；OAuth 与对象存储等敏感字段写入前需要它。
	if c.IsProduction() && c.StorageEncryptKey == "" {
		errs = append(errs, "STORAGE_ENCRYPT_KEY is required in production")
	}

	// 生产无 Redis 时必须有显式 JWT_KEY，避免运行时回退到公开默认密钥。
	if c.IsProduction() && strings.TrimSpace(c.RedisHost) == "" {
		key := strings.TrimSpace(c.JWTKey)
		if key == "" || key == "default-secret" {
			errs = append(errs, "JWT_KEY is required in production when REDIS_HOST is empty")
		}
	}

	if c.StorageEncryptKey != "" && len(c.StorageEncryptKey) != 64 {
		errs = append(errs, "STORAGE_ENCRYPT_KEY must be a 64-char hex string")
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}
