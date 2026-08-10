package repository

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"GOSpeak/internal/config"
)

func TestSQLiteReadOnlyWorkerDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worker.db")
	writable, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatalf("open writable sqlite: %v", err)
	}
	if err := writable.Exec("CREATE TABLE IF NOT EXISTS worker_probe (id INTEGER)").Error; err != nil {
		t.Fatalf("seed writable sqlite: %v", err)
	}
	sqlDB, err := writable.DB()
	if err != nil {
		t.Fatalf("get sqlite conn: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close writable sqlite: %v", err)
	}

	cfg := &config.Config{
		DBType:      "SQLite",
		DBPath:      path,
		ClusterRole: "worker",
	}
	db, err := InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB worker: %v", err)
	}
	t.Cleanup(func() {
		if db != nil {
			if sqlDB, err := db.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
	})

	if err := db.Exec("CREATE TABLE forbidden (id INTEGER)").Error; err == nil {
		t.Fatal("expected read-only SQLite write to fail")
	}
}

func TestSQLiteLocalDSN(t *testing.T) {
	cases := []struct {
		name, base, journal, want string
	}{
		{
			name:    "plain path",
			base:    "db/app.db",
			journal: "DELETE",
			want:    "file:db/app.db?_pragma=busy_timeout(10000)&_pragma=journal_mode(DELETE)&_pragma=foreign_keys(1)",
		},
		{
			name:    "file prefix",
			base:    "file:/tmp/libsql.db",
			journal: "WAL",
			want:    "file:/tmp/libsql.db?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)",
		},
		{
			name:    "file prefix with query",
			base:    "file:/tmp/libsql.db?cache=shared",
			journal: "DELETE",
			want:    "file:/tmp/libsql.db?cache=shared&_pragma=busy_timeout(10000)&_pragma=journal_mode(DELETE)&_pragma=foreign_keys(1)",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := sqliteLocalDSN(tt.base, tt.journal); got != tt.want {
				t.Fatalf("sqliteLocalDSN(%q, %q) = %q, want %q", tt.base, tt.journal, got, tt.want)
			}
		})
	}
}

func TestReadOnlyDSNHelpers(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "postgres appends options",
			got:  postgresReadOnlyDSN("host=a user=b password=c dbname=myapp port=5432 sslmode=disable"),
			want: "host=a user=b password=c dbname=myapp port=5432 sslmode=disable options=-cdefault_transaction_read_only=on",
		},
		{
			name: "postgres preserves existing options",
			got:  postgresReadOnlyDSN("host=a options=-csearch_path=public"),
			want: "host=a options=-csearch_path=public -cdefault_transaction_read_only=on",
		},
		{
			name: "mysql appends session variable",
			got:  mysqlReadOnlyDSN("u:p@tcp(h:3306)/myapp?parseTime=True"),
			want: "u:p@tcp(h:3306)/myapp?parseTime=True&transaction_read_only=ON",
		},
		{
			name: "mysql preserves existing session variable",
			got:  mysqlReadOnlyDSN("u:p@tcp(h:3306)/myapp?transaction_read_only=ON"),
			want: "u:p@tcp(h:3306)/myapp?transaction_read_only=ON",
		},
		{
			name: "sqlite read only file",
			got:  sqliteReadOnlyDSN("/tmp/worker.db"),
			want: "file:/tmp/worker.db?mode=ro&_pragma=busy_timeout(10000)",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestDSNForReadReplica(t *testing.T) {
	cfg := &config.Config{
		DBType:       "PostgreSQL",
		DBHost:       "primary.example",
		DBPort:       "5432",
		DBUser:       "writer",
		DBPassword:   "pw",
		DBReadHost:   "replica.example",
		DBReadPort:   "5433",
		DBReadUser:   "reader",
		DBReadDBName: "gospeak",
		DBReadOnly:   true,
	}
	got := dsnForRole(cfg, PostgreSQL, DBReadRole)
	want := "host=replica.example user=reader password= dbname=gospeak port=5433 sslmode=disable"
	if got != want {
		t.Fatalf("dsnForRole = %q, want %q", got, want)
	}
}

func TestConnectSQLiteLocalFileWithDBDSN(t *testing.T) {
	path := filepath.Join(t.TempDir(), "libsql-local.db")
	cfg := &config.Config{
		DBType: "SQLite",
		DBDSN:  "file:" + path,
	}
	db, err := connectSQLite(cfg, DBWriteRole)
	if err != nil {
		t.Fatalf("connectSQLite local DB_DSN: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlite conn: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.Exec("CREATE TABLE probe (id INTEGER PRIMARY KEY, name TEXT)").Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := db.Exec("INSERT INTO probe (name) VALUES (?)", "libsql").Error; err != nil {
		t.Fatalf("insert: %v", err)
	}
	var name string
	if err := db.Raw("SELECT name FROM probe LIMIT 1").Scan(&name).Error; err != nil {
		t.Fatalf("query: %v", err)
	}
	if name != "libsql" {
		t.Fatalf("name = %q, want libsql", name)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected db file %s: %v", path, err)
	}
}

func TestConnectSQLiteTursoRemote(t *testing.T) {
	cfg := &config.Config{
		DBType: "SQLite",
		// libsql 驱动支持 file: URL，本地冒烟测试无需真实 Turso 端点。
		TursoDatabaseURL: "file:" + filepath.Join(t.TempDir(), "turso-local.db"),
	}
	db, err := connectSQLite(cfg, DBWriteRole)
	if err != nil {
		t.Fatalf("connectSQLite turso: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlite conn: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if got := sqlDB.Stats().MaxOpenConnections; got != 10 {
		t.Fatalf("Turso MaxOpenConnections = %d, want 10", got)
	}
	if err := db.Exec("CREATE TABLE probe (id INTEGER PRIMARY KEY, name TEXT)").Error; err != nil {
		t.Fatalf("create table via libsql dialector: %v", err)
	}
	if err := db.Exec("INSERT INTO probe (name) VALUES (?)", "turso").Error; err != nil {
		t.Fatalf("insert via libsql dialector: %v", err)
	}
	var name string
	if err := db.Raw("SELECT name FROM probe LIMIT 1").Scan(&name).Error; err != nil {
		t.Fatalf("query via libsql dialector: %v", err)
	}
	if name != "turso" {
		t.Fatalf("name = %q, want turso", name)
	}
}

func TestInitDBTursoAutoMigrate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "turso-migrate.db")
	cfg := &config.Config{
		DBType:           "SQLite",
		TursoDatabaseURL: "file:" + path,
	}
	db, err := InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB turso: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlite conn: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?", "table", "users").Scan(&count).Error; err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	if count == 0 {
		t.Fatal("expected AutoMigrate to create users table via libsql dialector")
	}
	if !db.Migrator().HasTable("users") {
		t.Fatal("expected users table to exist after AutoMigrate")
	}
}
