package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestNormalizeDBTypeAliases(t *testing.T) {
	cases := map[string]string{
		"":            "SQLite",
		"sqlite":      "SQLite",
		"PostgresSQL": "PostgreSQL",
		"postgresql":  "PostgreSQL",
		"postgres":    "PostgreSQL",
		"MYSQL":       "MySQL",
		"mysql":       "MySQL",
	}
	for in, want := range cases {
		if got := normalizeDBType(in); got != want {
			t.Fatalf("normalizeDBType(%q)=%q want %q", in, got, want)
		}
	}
}

func TestLoadDefaults(t *testing.T) {
	clearConfigEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ServerPort != "8998" {
		t.Fatalf("ServerPort=%q want 8998", cfg.ServerPort)
	}
	if cfg.DBType != "SQLite" {
		t.Fatalf("DBType=%q want SQLite", cfg.DBType)
	}
	if cfg.DBPath != "db/app.db" {
		t.Fatalf("DBPath=%q want db/app.db", cfg.DBPath)
	}
	if cfg.SFUProvider != "livekit" {
		t.Fatalf("SFUProvider=%q want livekit", cfg.SFUProvider)
	}
	if cfg.CORSOrigin != "*" {
		t.Fatalf("CORSOrigin=%q want *", cfg.CORSOrigin)
	}
	if Current() != cfg {
		t.Fatal("Current() should return loaded config")
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("SERVER_PORT", "70000")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid SERVER_PORT error")
	}
}

func TestLoadRejectsInvalidSFU(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("SFU_PROVIDER", "unknown")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid SFU_PROVIDER error")
	}
}

func TestLoadRejectsProductionWithoutStorageEncryptKey(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_ENV", "production")
	if _, err := Load(); err == nil {
		t.Fatal("expected production STORAGE_ENCRYPT_KEY error")
	}
}

func TestLoadAcceptsProductionWithStorageEncryptKey(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("STORAGE_ENCRYPT_KEY", strings.Repeat("ab", 32))
	t.Setenv("JWT_KEY", "test-production-secret")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.IsProduction() || cfg.StorageEncryptKey == "" {
		t.Fatal("expected production config with storage encrypt key")
	}
}

func TestLoadRejectsProductionWithoutJWTKey(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("STORAGE_ENCRYPT_KEY", strings.Repeat("ab", 32))
	if _, err := Load(); err == nil {
		t.Fatal("expected production JWT_KEY error")
	}
}

func TestLoadAcceptsLegacyPostgresSQL(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DB_TYPE", "PostgresSQL")
	t.Setenv("DB_HOST", "db.example")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DBType != "PostgreSQL" {
		t.Fatalf("DBType=%q want PostgreSQL", cfg.DBType)
	}
	if cfg.DBPort != "5432" {
		t.Fatalf("DBPort=%q want 5432", cfg.DBPort)
	}
}

func TestLoadReadReplicaDefaults(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DB_TYPE", "PostgreSQL")
	t.Setenv("DB_HOST", "db.example")
	t.Setenv("DB_READ_HOST", "replica.example")
	t.Setenv("DB_READ_PASSWORD", "readpw")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.UsesReadReplica() {
		t.Fatal("expected UsesReadReplica true")
	}
	if cfg.DBReadHost != "replica.example" {
		t.Fatalf("DBReadHost=%q want replica.example", cfg.DBReadHost)
	}
	if cfg.DBReadPort != "5432" {
		t.Fatalf("DBReadPort=%q want 5432", cfg.DBReadPort)
	}
	if cfg.DBReadUser != "postgres" {
		t.Fatalf("DBReadUser=%q want postgres", cfg.DBReadUser)
	}
	if cfg.ReplicaLagThresholdDuration() != 5*time.Second {
		t.Fatalf("ReplicaLagThresholdDuration()=%v want 5s", cfg.ReplicaLagThresholdDuration())
	}
}

func TestLoadEnvFilesDoesNotOverrideProcessEnv(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".env.dev", []byte("SERVER_PORT=1111\nJWT_KEY=from-file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SERVER_PORT", "2222")
	// JWT_KEY intentionally unset so file can fill it
	_ = os.Unsetenv("JWT_KEY")

	LoadEnvFiles("dev")
	if got := os.Getenv("SERVER_PORT"); got != "2222" {
		t.Fatalf("process env should win, got %q", got)
	}
	if got := os.Getenv("JWT_KEY"); got != "from-file" {
		t.Fatalf("file should fill missing JWT_KEY, got %q", got)
	}
}

func TestLoadRejectsInvalidLogLevel(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("LOG_LEVEL", "verbose")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid LOG_LEVEL error")
	}
}

func TestLoadAcceptsLogConfig(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("LOG_OUTPUT", "stdout")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.LogLevel != "debug" || cfg.LogFormat != "json" || cfg.LogOutput != "stdout" {
		t.Fatalf("log cfg unexpected: %+v", cfg)
	}
}

func TestLoadAcceptsClusterRole(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("GOSPEAK_ROLE", "agent")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ClusterRole != "agent" {
		t.Fatalf("ClusterRole=%q want agent", cfg.ClusterRole)
	}
	if !cfg.IsAgent() || cfg.IsWorker() {
		t.Fatalf("agent role should be agent-only")
	}
}

func TestLoadRejectsInvalidClusterRole(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("GOSPEAK_ROLE", "controller")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid GOSPEAK_ROLE error")
	}
}

func TestLoadRejectsWorkerWithoutAgentURL(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("GOSPEAK_ROLE", "worker")
	if _, err := Load(); err == nil {
		t.Fatal("expected worker config error")
	}
}

func TestLoadAcceptsWorkerWithNodeSecret(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("GOSPEAK_ROLE", "worker")
	t.Setenv("CLUSTER_AGENT_URL", "http://agent:9000")
	t.Setenv("CLUSTER_AGENT_TOKEN", "admin-token")
	t.Setenv("CLUSTER_ADVERTISE_URL", "http://worker.example:8998")
	t.Setenv("CLUSTER_NODE_SECRET", "worker-secret")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load worker with secret: %v", err)
	}
	if cfg.ClusterNodeSecret != "worker-secret" {
		t.Fatalf("ClusterNodeSecret=%q want worker-secret", cfg.ClusterNodeSecret)
	}
}

func TestIsTurso(t *testing.T) {
	cases := []struct {
		name string
		cfg  *Config
		want bool
	}{
		{"turso url", &Config{TursoDatabaseURL: "libsql://db.turso.io"}, true},
		{"turso url and token", &Config{TursoDatabaseURL: "libsql://db.turso.io", TursoAuthToken: "tok"}, true},
		{"dbdsn libsql url", &Config{DBDSN: "libsql://db.turso.io?authToken=tok"}, true},
		{"dbdsn local file", &Config{DBDSN: "file:/tmp/app.db"}, false},
		{"dbpath only", &Config{DBPath: "db/app.db"}, false},
		{"empty", &Config{}, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.IsTurso(); got != tt.want {
				t.Fatalf("IsTurso() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEffectiveDSNPriority(t *testing.T) {
	cases := []struct {
		name string
		cfg  *Config
		want string
	}{
		{
			name: "turso url plus token",
			cfg:  &Config{TursoDatabaseURL: "libsql://db.turso.io", TursoAuthToken: "secret"},
			want: "libsql://db.turso.io?authToken=secret",
		},
		{
			name: "turso url with existing query",
			cfg:  &Config{TursoDatabaseURL: "libsql://db.turso.io?tls=1", TursoAuthToken: "secret"},
			want: "libsql://db.turso.io?tls=1&authToken=secret",
		},
		{
			name: "turso token url escaped",
			cfg:  &Config{TursoDatabaseURL: "libsql://db.turso.io", TursoAuthToken: "a/b+c="},
			want: "libsql://db.turso.io?authToken=a%2Fb%2Bc%3D",
		},
		{
			name: "turso url no token",
			cfg:  &Config{TursoDatabaseURL: "libsql://db.turso.io", DBDSN: "file:x", DBPath: "db/app.db"},
			want: "libsql://db.turso.io",
		},
		{
			name: "dbdsn over dbpath",
			cfg:  &Config{DBDSN: "file:/tmp/app.db", DBPath: "db/app.db"},
			want: "file:/tmp/app.db",
		},
		{
			name: "dbpath fallback",
			cfg:  &Config{DBPath: "db/app.db"},
			want: "db/app.db",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.EffectiveDSN(); got != tt.want {
				t.Fatalf("EffectiveDSN() = %q, want %q", got, tt.want)
			}
		})
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"APP_ENV", "DB_TYPE", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_PATH", "DB_DSN", "DB_WAL", "TURSO_DATABASE_URL", "TURSO_AUTH_TOKEN",
		"JWT_KEY", "JWT_KEY_TTL", "SFU_PROVIDER", "SERVER_PORT", "STATIC_DIR", "CORS_ORIGIN", "GIN_MODE",
		"LOG_LEVEL", "LOG_FORMAT", "LOG_OUTPUT", "LOG_FILE", "LOG_CALLER",
		"NATS_URL", "NATS_SUBJECT_PREFIX", "NATS_NAME", "NATS_CONNECT_TIMEOUT", "NATS_EMBEDDED_PORT", "STATE_STORE",
		"GOSPEAK_ROLE", "CLUSTER_NODE_ID", "CLUSTER_ADVERTISE_URL", "CLUSTER_AGENT_URL", "CLUSTER_AGENT_TOKEN",
		"CLUSTER_HEARTBEAT_INTERVAL", "CLUSTER_HEARTBEAT_TIMEOUT", "CLUSTER_MAX_SERVERS", "CLUSTER_MAX_ROOMS", "CLUSTER_LABELS",
		"EMAIL_ENABLED", "SMTP_HOST", "SMTP_PORT", "SMTP_USERNAME", "SMTP_PASSWORD", "SMTP_FROM", "SMTP_FROM_NAME",
		"EMAIL_CODE_TTL", "EMAIL_SEND_COOLDOWN", "EMAIL_CODE_SECRET",
		"STORAGE_TYPE", "STORAGE_ENDPOINT", "STORAGE_BUCKET", "STORAGE_REGION", "STORAGE_ACCESS_KEY",
		"STORAGE_SECRET_KEY", "STORAGE_PUBLIC_BASE_URL", "STORAGE_PATH_PREFIX", "STORAGE_ENCRYPT_KEY",
		"LIVEKIT_HOST", "LIVEKIT_KEY", "LIVEKIT_SECRET",
		"AGORA_APP_ID", "AGORA_APP_CERTIFICATE", "AGORA_HOST", "AGORA_CUSTOMER_ID", "AGORA_CUSTOMER_SECRET",
		"MEDIASOUP_BRIDGE_URL", "MEDIASOUP_HOST",
		"SRS_HOST", "SRS_API_PORT", "SRS_WHIP_URL", "SRS_SECRET", "SRS_PUBLIC_HOST",
		"DAILY_API_KEY", "DAILY_DOMAIN",
		"CF_APP_ID", "CF_APP_SECRET", "CF_STUN_URL",
	}
	for _, k := range keys {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}
	SetCurrent(nil)
}
