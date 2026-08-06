package config

import (
	"fmt"
	"net/url"
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

	// Turso (libSQL) 远程数据库配置（可选）
	// 优先级: TURSO_AUTH_TOKEN > DB_DSN?authToken=, TURSO_DATABASE_URL > DB_DSN
	TursoDatabaseURL string `env:"TURSO_DATABASE_URL" envDefault:""`
	TursoAuthToken   string `env:"TURSO_AUTH_TOKEN" envDefault:""`

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
	// MediaSoup/Daily 已禁用保留：实现文件仍在仓库，但 SFU_PROVIDER 不再接受。
	MediaSoupBridgeURL string `env:"MEDIASOUP_BRIDGE_URL" envDefault:"http://localhost:3012"`
	MediaSoupHost      string `env:"MEDIASOUP_HOST" envDefault:"localhost:3012"`
	SRSHost            string `env:"SRS_HOST" envDefault:"localhost"`
	SRSApiPort         string `env:"SRS_API_PORT" envDefault:"1985"`
	SRSWHIPURL         string `env:"SRS_WHIP_URL" envDefault:"/rtc/v1/whip/"`
	SRSSecret          string `env:"SRS_SECRET" envDefault:""`
	SRSPublicHost      string `env:"SRS_PUBLIC_HOST" envDefault:""`
	DailyAPIKey        string `env:"DAILY_API_KEY" envDefault:""`
	DailyDomain        string `env:"DAILY_DOMAIN" envDefault:""`
	CFAppID            string `env:"CF_APP_ID" envDefault:""`
	CFAppSecret        string `env:"CF_APP_SECRET" envDefault:""`
	CFStunURL          string `env:"CF_STUN_URL" envDefault:"stun.cloudflare.com:3478"`

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
	ClusterEntryURL          string `env:"CLUSTER_ENTRY_URL" envDefault:""`
	MetricsToken             string `env:"METRICS_TOKEN" envDefault:""`

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
// IsTurso returns true when Turso (libSQL) remote database is configured.
func (c *Config) IsTurso() bool {
	return c.TursoDatabaseURL != "" || strings.HasPrefix(c.DBDSN, "libsql://")
}

// EffectiveDSN returns the resolved SQLite/libSQL DSN.
// Priority: TURSO_DATABASE_URL (+ TURSO_AUTH_TOKEN) > DB_DSN > file:DB_PATH.
func (c *Config) EffectiveDSN() string {
	if c.TursoDatabaseURL != "" {
		dsn := c.TursoDatabaseURL
		if c.TursoAuthToken != "" {
			sep := "?"
			if strings.Contains(dsn, "?") {
				sep = "&"
			}
			dsn += sep + "authToken=" + url.QueryEscape(c.TursoAuthToken)
		}
		return dsn
	}
	if c.DBDSN != "" {
		return c.DBDSN
	}
	return c.DBPath
}

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
