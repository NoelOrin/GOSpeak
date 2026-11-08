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
	case "livekit":
	default:
		errs = append(errs, fmt.Sprintf("SFU_PROVIDER %q unsupported (only livekit)", c.SFUProvider))
	}

	switch c.StorageType {
	case "local", "s3":
	default:
		errs = append(errs, fmt.Sprintf("STORAGE_TYPE %q unsupported (local|s3)", c.StorageType))
	}

	if c.AccessCookieName == "" {
		errs = append(errs, "ACCESS_COOKIE_NAME is required")
	}
	if c.RefreshCookieName == "" {
		errs = append(errs, "REFRESH_COOKIE_NAME is required")
	}
	if c.AccessCookieName == c.RefreshCookieName {
		errs = append(errs, "ACCESS_COOKIE_NAME and REFRESH_COOKIE_NAME must differ")
	}
	if c.CookiePath == "" || !strings.HasPrefix(c.CookiePath, "/") {
		errs = append(errs, "COOKIE_PATH must start with /")
	}
	switch c.CookieSecure {
	case "auto", "true", "false":
	default:
		errs = append(errs, fmt.Sprintf("COOKIE_SECURE %q unsupported (auto|true|false)", c.CookieSecure))
	}
	switch c.CookieSameSite {
	case "lax", "strict", "none":
	default:
		errs = append(errs, fmt.Sprintf("COOKIE_SAMESITE %q unsupported (lax|strict|none)", c.CookieSameSite))
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

	if c.EmailEnabled && strings.TrimSpace(c.EmailCodeSecret) == "" {
		errs = append(errs, "EMAIL_CODE_SECRET is required when EMAIL_ENABLED=true")
	}

	// 生产环境强制要求加密密钥；OAuth 与对象存储等敏感字段写入前需要它。
	if c.IsProduction() && c.StorageEncryptKey == "" {
		errs = append(errs, "STORAGE_ENCRYPT_KEY is required in production")
	}

	// 生产必须有显式 JWT_KEY，避免运行时回退到公开默认密钥。
	if c.IsProduction() {
		key := strings.TrimSpace(c.JWTKey)
		if key == "" || key == "default-secret" {
			errs = append(errs, "JWT_KEY is required in production")
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
