package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/caarlos0/env/v9"
	"github.com/joho/godotenv"
)

// Config 集中承载服务端环境配置。
// 加载优先级：进程环境变量 > 环境专属文件(.env.dev/.env.prod) > 通用 .env > 字段默认值。
type Config struct {
	AppEnv string `env:"APP_ENV" envDefault:""`

	DBType     string `env:"DB_TYPE" envDefault:"SQLite"`
	DBHost     string `env:"DB_HOST" envDefault:"localhost"`
	DBPort     string `env:"DB_PORT" envDefault:""`
	DBUser     string `env:"DB_USER" envDefault:""`
	DBPassword string `env:"DB_PASSWORD" envDefault:""`
	DBPath     string `env:"DB_PATH" envDefault:"db/app.db"`
	DBDSN      string `env:"DB_DSN" envDefault:""`
	DBWAL      bool   `env:"DB_WAL" envDefault:"false"`

	JWTKey    string `env:"JWT_KEY" envDefault:"default-secret"`
	JWTKeyTTL string `env:"JWT_KEY_TTL" envDefault:"24h"`

	SFUProvider         string `env:"SFU_PROVIDER" envDefault:"livekit"`
	LiveKitHost         string `env:"LIVEKIT_HOST" envDefault:""`
	LiveKitKey          string `env:"LIVEKIT_KEY" envDefault:""`
	LiveKitSecret       string `env:"LIVEKIT_SECRET" envDefault:""`
	AgoraAppID          string `env:"AGORA_APP_ID" envDefault:""`
	AgoraAppCertificate string `env:"AGORA_APP_CERTIFICATE" envDefault:""`
	AgoraHost           string `env:"AGORA_HOST" envDefault:""`
	AgoraCustomerID     string `env:"AGORA_CUSTOMER_ID" envDefault:""`
	AgoraCustomerSecret string `env:"AGORA_CUSTOMER_SECRET" envDefault:""`
	MediaSoupBridgeURL  string `env:"MEDIASOUP_BRIDGE_URL" envDefault:"http://localhost:3012"`
	MediaSoupHost       string `env:"MEDIASOUP_HOST" envDefault:"localhost:3012"`
	SRSHost             string `env:"SRS_HOST" envDefault:"localhost"`
	SRSApiPort          string `env:"SRS_API_PORT" envDefault:"1985"`
	SRSWHIPURL          string `env:"SRS_WHIP_URL" envDefault:"/rtc/v1/whip/"`
	SRSSecret           string `env:"SRS_SECRET" envDefault:""`
	SRSPublicHost       string `env:"SRS_PUBLIC_HOST" envDefault:""`
	DailyAPIKey         string `env:"DAILY_API_KEY" envDefault:""`
	DailyDomain         string `env:"DAILY_DOMAIN" envDefault:""`
	CFAppID             string `env:"CF_APP_ID" envDefault:""`
	CFAppSecret         string `env:"CF_APP_SECRET" envDefault:""`
	CFStunURL           string `env:"CF_STUN_URL" envDefault:"stun.cloudflare.com:3478"`

	ServerPort       string `env:"SERVER_PORT" envDefault:"8998"`
	StaticDir        string `env:"STATIC_DIR" envDefault:""`
	CORSOrigin       string `env:"CORS_ORIGIN" envDefault:"*"`
	GinMode          string `env:"GIN_MODE" envDefault:""`
	WSAllowedOrigins string `env:"WS_ALLOWED_ORIGINS" envDefault:""`

	// 日志
	LogLevel  string `env:"LOG_LEVEL" envDefault:""`  // trace|debug|info|warn|error；空则 dev=debug / prod=info
	LogFormat string `env:"LOG_FORMAT" envDefault:""` // text|json；空则 dev=text / prod=json
	LogOutput string `env:"LOG_OUTPUT" envDefault:""` // stdout|stderr|file|both；默认 stdout
	LogFile   string `env:"LOG_FILE" envDefault:""`   // file/both 时路径，默认 logs/app.log
	LogCaller bool   `env:"LOG_CALLER" envDefault:"false"`

	RedisHost     string `env:"REDIS_HOST" envDefault:""`
	RedisPort     string `env:"REDIS_PORT" envDefault:"6379"`
	RedisPassword string `env:"REDIS_PASSWORD" envDefault:""`
	RedisDB       string `env:"REDIS_DB" envDefault:"0"`

	NATSURL            string `env:"NATS_URL" envDefault:""`
	NATSSubjectPrefix  string `env:"NATS_SUBJECT_PREFIX" envDefault:"gospeak"`
	NATSName           string `env:"NATS_NAME" envDefault:""`
	NATSConnectTimeout string `env:"NATS_CONNECT_TIMEOUT" envDefault:"2s"`
	NATSEmbeddedPort   string `env:"NATS_EMBEDDED_PORT" envDefault:""`
	NATSUser           string `env:"NATS_USER" envDefault:""`
	NATSPassword       string `env:"NATS_PASSWORD" envDefault:""`
	NATSToken          string `env:"NATS_TOKEN" envDefault:""`
	NATSCredsFile      string `env:"NATS_CREDS_FILE" envDefault:""`
	NATSTLS            bool   `env:"NATS_TLS" envDefault:"false"`
	StateStore         string `env:"STATE_STORE" envDefault:"auto"`

	ClusterRole              string `env:"GOSPEAK_ROLE" envDefault:"all"`
	ClusterNodeID            string `env:"CLUSTER_NODE_ID" envDefault:""`
	ClusterAdvertiseURL      string `env:"CLUSTER_ADVERTISE_URL" envDefault:""`
	ClusterAgentURL          string `env:"CLUSTER_AGENT_URL" envDefault:""`
	ClusterAgentToken        string `env:"CLUSTER_AGENT_TOKEN" envDefault:""`
	ClusterHeartbeatInterval string `env:"CLUSTER_HEARTBEAT_INTERVAL" envDefault:"5s"`
	ClusterHeartbeatTimeout  string `env:"CLUSTER_HEARTBEAT_TIMEOUT" envDefault:"30s"`
	ClusterMaxServers        int    `env:"CLUSTER_MAX_SERVERS" envDefault:"100"`
	ClusterMaxRooms          int    `env:"CLUSTER_MAX_ROOMS" envDefault:"1000"`
	ClusterLabels            string `env:"CLUSTER_LABELS" envDefault:""`

	EmailEnabled      bool   `env:"EMAIL_ENABLED" envDefault:"false"`
	SMTPHost          string `env:"SMTP_HOST" envDefault:""`
	SMTPPort          string `env:"SMTP_PORT" envDefault:"587"`
	SMTPUsername      string `env:"SMTP_USERNAME" envDefault:""`
	SMTPPassword      string `env:"SMTP_PASSWORD" envDefault:""`
	SMTPFrom          string `env:"SMTP_FROM" envDefault:""`
	SMTPFromName      string `env:"SMTP_FROM_NAME" envDefault:"GoSpeak"`
	EmailCodeTTL      string `env:"EMAIL_CODE_TTL" envDefault:"10m"`
	EmailSendCooldown string `env:"EMAIL_SEND_COOLDOWN" envDefault:"60s"`
	EmailCodeSecret   string `env:"EMAIL_CODE_SECRET" envDefault:""`

	StorageType          string `env:"STORAGE_TYPE" envDefault:"local"`
	StorageEndpoint      string `env:"STORAGE_ENDPOINT" envDefault:""`
	StorageBucket        string `env:"STORAGE_BUCKET" envDefault:""`
	StorageRegion        string `env:"STORAGE_REGION" envDefault:""`
	StorageAccessKey     string `env:"STORAGE_ACCESS_KEY" envDefault:""`
	StorageSecretKey     string `env:"STORAGE_SECRET_KEY" envDefault:""`
	StoragePublicBaseURL string `env:"STORAGE_PUBLIC_BASE_URL" envDefault:""`
	StoragePathPrefix    string `env:"STORAGE_PATH_PREFIX" envDefault:"uploads/"`
	StorageEncryptKey    string `env:"STORAGE_ENCRYPT_KEY" envDefault:""`
}

var current *Config

// Current 返回最近一次成功 Load 的配置；未加载时返回 nil。
func Current() *Config {
	return current
}

// SetCurrent 供测试或特殊启动路径注入配置。
func SetCurrent(cfg *Config) {
	current = cfg
}

// LoadEnvFiles 按优先级加载 env 文件到进程环境。
// 已存在的进程环境变量不会被覆盖：process > env-specific > .env。
func LoadEnvFiles(appEnv string) {
	files := make([]string, 0, 2)
	switch strings.ToLower(strings.TrimSpace(appEnv)) {
	case "dev", "development":
		files = append(files, ".env.dev")
	case "prod", "production", "":
		files = append(files, ".env.prod")
	default:
		// 允许自定义环境名，优先尝试 .env.<name>
		files = append(files, ".env."+strings.ToLower(strings.TrimSpace(appEnv)))
	}
	files = append(files, ".env")

	for _, path := range files {
		vals, err := godotenv.Read(path)
		if err != nil {
			continue
		}
		for key, value := range vals {
			if _, exists := os.LookupEnv(key); !exists {
				_ = os.Setenv(key, value)
			}
		}
	}
}

// Load 从环境变量解析配置、规范化并校验。
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse env: %w", err)
	}
	cfg.normalize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	current = cfg
	return cfg, nil
}

// MustLoad 同 Load，失败时 panic。
func MustLoad() *Config {
	cfg, err := Load()
	if err != nil {
		panic(fmt.Sprintf("invalid configuration: %v", err))
	}
	return cfg
}

// IsProduction 判断是否生产环境。
func (c *Config) IsProduction() bool {
	switch strings.ToLower(strings.TrimSpace(c.AppEnv)) {
	case "prod", "production":
		return true
	default:
		return false
	}
}

// GetLogLevel / GetLogFormat / GetLogOutput / GetLogFile / GetLogCaller
// 供 logger 包通过接口读取，避免循环依赖。
func (c *Config) GetLogLevel() string  { return c.LogLevel }
func (c *Config) GetLogFormat() string { return c.LogFormat }
func (c *Config) GetLogOutput() string { return c.LogOutput }
func (c *Config) GetLogFile() string   { return c.LogFile }
func (c *Config) GetLogCaller() bool   { return c.LogCaller }

// LoggerOptions 构造 logger 初始化选项。
func (c *Config) LoggerOptions() (level, format, output, file string, caller, production bool) {
	return c.LogLevel, c.LogFormat, c.LogOutput, c.LogFile, c.LogCaller, c.IsProduction()
}

// IsDevelopment 判断是否开发环境。
func (c *Config) IsDevelopment() bool {
	switch strings.ToLower(strings.TrimSpace(c.AppEnv)) {
	case "dev", "development":
		return true
	default:
		return false
	}
}

// JWTKeyTTLDuration 解析 JWT 密钥轮换周期。
func (c *Config) JWTKeyTTLDuration() time.Duration {
	d, err := time.ParseDuration(strings.TrimSpace(c.JWTKeyTTL))
	if err != nil || d <= 0 {
		return 24 * time.Hour
	}
	return d
}

// RedisDBIndex 解析 Redis DB 序号。
func (c *Config) RedisDBIndex() int {
	n, err := strconv.Atoi(strings.TrimSpace(c.RedisDB))
	if err != nil {
		return 0
	}
	return n
}

// NATSConnectTimeoutDuration 解析 NATS 连接超时。
func (c *Config) NATSConnectTimeoutDuration() (time.Duration, error) {
	return time.ParseDuration(strings.TrimSpace(c.NATSConnectTimeout))
}

// ClusterHeartbeatIntervalDuration 解析节点心跳间隔。
func (c *Config) ClusterHeartbeatIntervalDuration() time.Duration {
	d, err := time.ParseDuration(strings.TrimSpace(c.ClusterHeartbeatInterval))
	if err != nil || d <= 0 {
		return 5 * time.Second
	}
	return d
}

// ClusterHeartbeatTimeoutDuration 解析节点离线判定超时。
func (c *Config) ClusterHeartbeatTimeoutDuration() time.Duration {
	d, err := time.ParseDuration(strings.TrimSpace(c.ClusterHeartbeatTimeout))
	if err != nil || d <= 0 {
		return 30 * time.Second
	}
	return d
}

// IsAgent 返回当前进程是否承担 Agent 控制面职责。
func (c *Config) IsAgent() bool {
	return c.ClusterRole == "agent" || c.ClusterRole == "all"
}

// IsWorker 返回当前进程是否承担 Worker 数据面职责。
func (c *Config) IsWorker() bool {
	return c.ClusterRole == "worker" || c.ClusterRole == "all"
}

// WSAllowedOriginsList 返回 WebSocket Origin 白名单。
// 未配置 WS_ALLOWED_ORIGINS 时默认只允许同源握手；配置 "*" 表示允许任意来源。
func (c *Config) WSAllowedOriginsList() []string {
	if strings.TrimSpace(c.WSAllowedOrigins) == "" {
		return nil
	}
	raw := strings.Split(c.WSAllowedOrigins, ",")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func (c *Config) normalize() {
	c.AppEnv = strings.TrimSpace(c.AppEnv)
	c.DBType = normalizeDBType(c.DBType)
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
	c.ClusterNodeID = strings.TrimSpace(c.ClusterNodeID)
	c.ClusterAdvertiseURL = strings.TrimSpace(c.ClusterAdvertiseURL)
	c.ClusterAgentURL = strings.TrimSpace(c.ClusterAgentURL)
	c.ClusterAgentToken = strings.TrimSpace(c.ClusterAgentToken)
	c.ClusterHeartbeatInterval = strings.TrimSpace(c.ClusterHeartbeatInterval)
	c.ClusterHeartbeatTimeout = strings.TrimSpace(c.ClusterHeartbeatTimeout)
	c.ClusterLabels = strings.TrimSpace(c.ClusterLabels)
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
	if c.ClusterHeartbeatInterval == "" {
		c.ClusterHeartbeatInterval = "5s"
	}
	if c.ClusterHeartbeatTimeout == "" {
		c.ClusterHeartbeatTimeout = "30s"
	}

	c.LogLevel = strings.ToLower(strings.TrimSpace(c.LogLevel))
	c.LogFormat = strings.ToLower(strings.TrimSpace(c.LogFormat))
	c.LogOutput = strings.ToLower(strings.TrimSpace(c.LogOutput))
	c.LogFile = strings.TrimSpace(c.LogFile)
}

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
	case "livekit", "srs", "mediasoup", "agora", "daily", "cloudflare":
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

	// 生产环境 + S3 存储时强制要求加密密钥；local 存储可延后到首次加密时由 storage 层处理。
	if c.IsProduction() && c.StorageType == "s3" && c.StorageEncryptKey == "" {
		errs = append(errs, "STORAGE_ENCRYPT_KEY is required in production when STORAGE_TYPE=s3")
	}

	if c.StorageEncryptKey != "" && len(c.StorageEncryptKey) != 64 {
		errs = append(errs, "STORAGE_ENCRYPT_KEY must be a 64-char hex string")
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
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
