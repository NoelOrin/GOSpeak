package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
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

	JWTAccessTTL  string `env:"JWT_ACCESS_TTL" envDefault:"15m"`
	JWTRefreshTTL string `env:"JWT_REFRESH_TTL" envDefault:"168h"`

	// BcryptCost 密码哈希工作因子，默认 12。
	BcryptCost int `env:"BCRYPT_COST" envDefault:"12"`

	SFUProvider   string `env:"SFU_PROVIDER" envDefault:"livekit"`
	// LiveKitHost 是后端访问 LiveKit 的地址（admin API / RoomService），
	// 容器部署填内网服务名 ws://livekit:7880 即可。
	LiveKitHost string `env:"LIVEKIT_HOST" envDefault:""`
	// LiveKitPublicURL 是下发给浏览器作为 livekit-client 连接目标的地址。
	// 留空（默认）= 走应用内置的同源反代：按请求 Host 推导 wss://<域名>/livekit，
	// 因此 LiveKit 无需独立域名、证书或反代规则。
	// 仅当 LiveKit 有独立对外地址（独立域名 / LiveKit Cloud）时才显式填写。
	LiveKitPublicURL string `env:"LIVEKIT_PUBLIC_URL" envDefault:""`
	LiveKitKey       string `env:"LIVEKIT_KEY" envDefault:""`
	LiveKitSecret    string `env:"LIVEKIT_SECRET" envDefault:""`

	// OAuthAutoCreateUser 控制 OAuth 回调时自动创建未绑定用户；关闭后仅允许已绑定账号登录。
	OAuthAutoCreateUser bool `env:"OAUTH_AUTO_CREATE_USER" envDefault:"true"`

	ServerPort       string `env:"SERVER_PORT" envDefault:"8998"`
	StaticDir        string `env:"STATIC_DIR" envDefault:""`
	CORSOrigin       string `env:"CORS_ORIGIN" envDefault:"*"`
	GinMode          string `env:"GIN_MODE" envDefault:""`
	WSAllowedOrigins string `env:"WS_ALLOWED_ORIGINS" envDefault:""`
	TrustedProxies   string `env:"TRUSTED_PROXIES" envDefault:""`

	// TLS 直跑 HTTPS + HTTP/2：两者同时配置时启用；未配置时保持明文 HTTP/1.1。
	TLSCertFile string `env:"TLS_CERT" envDefault:""`
	TLSKeyFile  string `env:"TLS_KEY" envDefault:""`

	// 鉴权 Cookie 配置：access/refresh token 均以 HttpOnly Cookie 承载。
	AccessCookieName  string `env:"ACCESS_COOKIE_NAME" envDefault:"gospeak_token"`
	RefreshCookieName string `env:"REFRESH_COOKIE_NAME" envDefault:"gospeak_refresh_token"`
	CookieDomain      string `env:"COOKIE_DOMAIN" envDefault:""`
	CookiePath        string `env:"COOKIE_PATH" envDefault:"/"`
	CookieSecure      string `env:"COOKIE_SECURE" envDefault:"auto"`  // auto|true|false
	CookieSameSite    string `env:"COOKIE_SAMESITE" envDefault:"lax"` // lax|strict|none

	// 日志
	LogLevel  string `env:"LOG_LEVEL" envDefault:""`  // trace|debug|info|warn|error；空则 dev=debug / prod=info
	LogFormat string `env:"LOG_FORMAT" envDefault:""` // text|json；空则 dev=text / prod=json
	LogOutput string `env:"LOG_OUTPUT" envDefault:""` // stdout|stderr|file|both；默认 stdout
	LogFile   string `env:"LOG_FILE" envDefault:""`   // file/both 时路径，默认 logs/app.log
	LogCaller bool   `env:"LOG_CALLER" envDefault:"false"`

	NATSSubjectPrefix  string `env:"NATS_SUBJECT_PREFIX" envDefault:"gospeak"`
	NATSName           string `env:"NATS_NAME" envDefault:""`
	NATSConnectTimeout string `env:"NATS_CONNECT_TIMEOUT" envDefault:"2s"`
	NATSEmbeddedPort   string `env:"NATS_EMBEDDED_PORT" envDefault:""`
	NATSWAL            string `env:"NATS_WAL_PATH" envDefault:""`

	MetricsToken string `env:"METRICS_TOKEN" envDefault:""`

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
	StorageLocalDir      string `env:"STORAGE_LOCAL_DIR" envDefault:"uploads"`
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
//
// 查找顺序（先找到先用，后者只填补前者未设置的键）：
//  1. GOSPEAK_ENV_FILE 指定的文件（显式覆盖，优先级最高）
//  2. 当前工作目录下的 .env.<env> / .env（本地开发：app/server/）
//  3. 仓库根的 deploy/.env（统一配置文件，compose 与应用共用）
//
// 统一到 deploy/.env 后，本地 `pnpm dev` 无需再维护 app/server/.env。
func LoadEnvFiles(appEnv string) {
	files := make([]string, 0, 4)

	// 显式指定优先
	if explicit := strings.TrimSpace(os.Getenv("GOSPEAK_ENV_FILE")); explicit != "" {
		files = append(files, explicit)
	}

	var envFile string
	switch strings.ToLower(strings.TrimSpace(appEnv)) {
	case "dev", "development":
		envFile = ".env.dev"
	case "prod", "production", "":
		envFile = ".env.prod"
	default:
		// 允许自定义环境名，优先尝试 .env.<name>
		envFile = ".env." + strings.ToLower(strings.TrimSpace(appEnv))
	}

	// 工作目录（app/server 或二进制所在目录）
	files = append(files, envFile, ".env")

	// 仓库根的 deploy/.env：从工作目录逐级向上找，命中含 deploy/.env 的根即止。
	files = append(files, findRepoDeployEnv()...)

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

// findRepoDeployEnv 从当前工作目录向上查找仓库根下的 deploy/.env。
// 上溯层数按最深场景（app/server/internal/config 等）留足余量，
// 一旦到达文件系统根即停止，不会无限遍历。
func findRepoDeployEnv() []string {
	dir, err := os.Getwd()
	if err != nil {
		return nil
	}
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "deploy", ".env")
		if _, err := os.Stat(candidate); err == nil {
			return []string{candidate}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil
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

// JWTAccessTTLDuration 解析 access token 有效期。
func (c *Config) JWTAccessTTLDuration() time.Duration {
	d, err := time.ParseDuration(strings.TrimSpace(c.JWTAccessTTL))
	if err != nil || d <= 0 {
		return 15 * time.Minute
	}
	return d
}

// JWTRefreshTTLDuration 解析 refresh token 有效期。
func (c *Config) JWTRefreshTTLDuration() time.Duration {
	d, err := time.ParseDuration(strings.TrimSpace(c.JWTRefreshTTL))
	if err != nil || d <= 0 {
		return 7 * 24 * time.Hour
	}
	return d
}

// NATSConnectTimeoutDuration 解析 NATS 连接超时。
func (c *Config) NATSConnectTimeoutDuration() (time.Duration, error) {
	return time.ParseDuration(strings.TrimSpace(c.NATSConnectTimeout))
}

// NATSWALPath 返回断线事件 WAL 文件路径；空表示禁用磁盘持久化。
func (c *Config) NATSWALPath() string {
	return strings.TrimSpace(c.NATSWAL)
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

// TrustedProxiesList 返回信任的代理 IP/CIDR 列表；为空时调用方回退默认值。
func (c *Config) TrustedProxiesList() []string {
	if strings.TrimSpace(c.TrustedProxies) == "" {
		return nil
	}
	raw := strings.Split(c.TrustedProxies, ",")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}
