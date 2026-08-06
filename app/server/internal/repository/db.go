package repository

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	_ "github.com/tursodatabase/libsql-client-go/libsql"

	"GOSpeak/internal/config"
	"GOSpeak/internal/model"
)

type DatabaseEnum string

const (
	PostgreSQL DatabaseEnum = "PostgreSQL"
	SQLite     DatabaseEnum = "SQLite"
	MySQL      DatabaseEnum = "MySQL"
)

// libsqlDriverName 是 tursodatabase/libsql-client-go 注册的 database/sql 驱动名。
const libsqlDriverName = "libsql"

var (
	dbMu     sync.RWMutex
	globalDB *gorm.DB
)

// setDB atomically writes the global DB handle.

// setDB atomically writes the global DB handle.
func setDB(d *gorm.DB) {
	dbMu.Lock()
	globalDB = d
	dbMu.Unlock()
}

// getDB returns the global DB handle.

// getDB returns the global DB handle.
func getDB() *gorm.DB {
	dbMu.RLock()
	defer dbMu.RUnlock()
	return globalDB
}

// DBStats 返回连接池统计，供监控与 readiness 使用。
func DBStats() sql.DBStats {
	db := getDB()
	if db == nil {
		return sql.DBStats{}
	}
	sqlDB, err := db.DB()
	if err != nil {
		return sql.DBStats{}
	}
	return sqlDB.Stats()
}

// DBPing 探测底层连接池可用性，供健康检查使用。
func DBPing() error {
	db := getDB()
	if db == nil {
		return fmt.Errorf("db not initialized")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

func InitDB(cfg *config.Config) (*gorm.DB, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	dbType := DatabaseEnum(cfg.DBType)
	var err error
	var db *gorm.DB
	switch dbType {
	case PostgreSQL:
		db, err = connectPostgreSQL(cfg)
	case SQLite:
		db, err = connectSQLite(cfg)
	case MySQL:
		db, err = connectMySQL(cfg)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
	if err != nil {
		panic(fmt.Sprintf("数据库连接失败 [%s]: %v", dbType, err))
	}
	setDB(db)

	sqlDB, err := db.DB()
	if err != nil {
		panic(fmt.Sprintf("获取数据库实例失败 [%s]: %v", dbType, err))
	}
	if err = sqlDB.Ping(); err != nil {
		panic(fmt.Sprintf("数据库 Ping 失败 [%s]: %v", dbType, err))
	}

	// 仅严格 worker 跳过 schema 迁移；默认 all/agent 必须负责建表。
	if cfg.ClusterRole == model.ClusterRoleWorker {
		// Worker 数据面不负责 schema 迁移，避免对共享权威库产生写操作。
		return db, nil
	}

	if err := migrateOldSFUConfig(db); err != nil {
		return nil, err
	}
	if err := migrateStorageConfigSchema(db); err != nil {
		return nil, err
	}
	if err := migrateBotTokensSchema(db); err != nil {
		return nil, err
	}
	if err := migrateEmailVerificationCodesSchema(db); err != nil {
		return nil, err
	}

	if err := autoMigrate(); err != nil {
		return nil, err
	}

	if err := migrateUserSearchIndexes(db, dbType); err != nil {
		return nil, err
	}
	if err := migrateMessageAuthorUUID(db); err != nil {
		return nil, err
	}

	if err := migrateDomainCUID2(db); err != nil {
		return nil, err
	}

	if err := migrateRoomPasswords(db); err != nil {
		return nil, err
	}

	return db, nil
}

func connectPostgreSQL(cfg *config.Config) (*gorm.DB, error) {
	dsn := cfg.DBDSN
	if dsn == "" {
		user := cfg.DBUser
		if user == "" {
			user = "postgres"
		}
		password := cfg.DBPassword
		if password == "" {
			password = "postgres"
		}
		host := cfg.DBHost
		if host == "" {
			host = "localhost"
		}
		port := cfg.DBPort
		if port == "" {
			port = "5432"
		}
		dsn = fmt.Sprintf(
			"host=%s user=%s password=%s dbname=myapp port=%s sslmode=disable",
			host, user, password, port,
		)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := configureDBPool(db, cfg); err != nil {
		return nil, err
	}
	return db, nil
}

func connectSQLite(cfg *config.Config) (*gorm.DB, error) {
	// Worker 模式只读本地文件，不走 Turso。
	if cfg.ClusterRole == model.ClusterRoleWorker {
		path := cfg.DBPath
		if path == "" {
			return nil, fmt.Errorf("DB_PATH is required for worker SQLite mode")
		}
		dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(10000)", path)
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		if err != nil {
			return nil, err
		}
		if err := configureDBPool(db, cfg); err != nil {
			return nil, err
		}
		return db, nil
	}

	// Turso 远程模式（libSQL URL）：远程不支持 WAL/foreign_keys pragma，DSN 原样直通。
	if cfg.IsTurso() {
		// 复用 glebarez 的 GORM dialector，仅替换 database/sql 驱动为 libsql。
		db, err := gorm.Open(&sqlite.Dialector{DriverName: libsqlDriverName, DSN: cfg.EffectiveDSN()}, &gorm.Config{})
		if err != nil {
			return nil, err
		}
		// Turso 远程延迟高于本地文件，连接池放宽。
		if err := configureDBPoolForTurso(db); err != nil {
			return nil, err
		}
		return db, nil
	}

	// 本地模式：DB_PATH 或 DB_DSN（libSQL 文件），自动补 file: 前缀与 pragma。
	base := cfg.EffectiveDSN()
	if !strings.HasPrefix(base, "file:") {
		path := strings.SplitN(base, "?", 2)[0]
		if path != "" {
			if dir := parentDir(path); dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return nil, fmt.Errorf("failed to create db directory: %w", err)
				}
			}
		}
	}
	// glebarez/modernc 使用 _pragma=...，旧的 _journal_mode/_busy_timeout 不会生效。
	// WAL + busy_timeout 可降低热重载/并发写时的 SQLITE_BUSY，并支持多连接并发读。
	// 非 WAL 时连接池会强制 MaxOpenConns=1，避免 DELETE 日志模式写锁竞争。
	journalMode := "DELETE"
	if cfg.DBWAL {
		journalMode = "WAL"
	}
	dsn := sqliteLocalDSN(base, journalMode)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := configureDBPool(db, cfg); err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if _, err := sqlDB.Exec(fmt.Sprintf("PRAGMA journal_mode=%s", journalMode)); err != nil {
		return nil, fmt.Errorf("set journal_mode=%s: %w", journalMode, err)
	}
	if _, err := sqlDB.Exec("PRAGMA busy_timeout=10000"); err != nil {
		return nil, fmt.Errorf("set busy_timeout: %w", err)
	}
	if cfg.DBWAL {
		if _, err := sqlDB.Exec("PRAGMA synchronous=NORMAL"); err != nil {
			return nil, fmt.Errorf("set synchronous=NORMAL: %w", err)
		}
	} else {
		if _, err := sqlDB.Exec("PRAGMA synchronous=FULL"); err != nil {
			return nil, fmt.Errorf("set synchronous=FULL: %w", err)
		}
	}
	if _, err := sqlDB.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("set foreign_keys=ON: %w", err)
	}
	return db, nil
}

// sqliteLocalDSN 组装本地 SQLite/libSQL 文件 DSN。
// 支持裸路径、file: 前缀以及已带查询参数的 DSN。
func sqliteLocalDSN(base, journalMode string) string {
	if !strings.HasPrefix(base, "file:") {
		base = "file:" + base
	}
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return fmt.Sprintf(
		"%s%s_pragma=busy_timeout(10000)&_pragma=journal_mode(%s)&_pragma=foreign_keys(1)",
		base, sep, journalMode,
	)
}

func connectMySQL(cfg *config.Config) (*gorm.DB, error) {
	dsn := cfg.DBDSN
	if dsn == "" {
		user := cfg.DBUser
		if user == "" {
			user = "root"
		}
		host := cfg.DBHost
		if host == "" {
			host = "localhost"
		}
		port := cfg.DBPort
		if port == "" {
			port = "3306"
		}
		dsn = fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/myapp?charset=utf8mb4&parseTime=True&loc=Local",
			user, cfg.DBPassword, host, port,
		)
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := configureDBPool(db, cfg); err != nil {
		return nil, err
	}
	return db, nil
}

// 连接池写死在代码内，不提供环境变量覆盖。
// SQLite 非 WAL：单连接；WAL：小连接池支持并发读。
// PostgreSQL/MySQL：固定生产向默认值。

// 连接池写死在代码内，不提供环境变量覆盖。
// SQLite 非 WAL：单连接；WAL：小连接池支持并发读。
// PostgreSQL/MySQL：固定生产向默认值。
const (
	sqliteWALMaxOpenConns = 4
	sqliteWALMaxIdleConns = 4

	rdbmsMaxOpenConns = 25
	rdbmsMaxIdleConns = 10
)

var (
	rdbmsConnMaxLifetime = 30 * time.Minute
	rdbmsConnMaxIdleTime = 5 * time.Minute
)

type dbPoolConfig struct {
	maxOpenConns    int
	maxIdleConns    int
	connMaxLifetime time.Duration
	connMaxIdleTime time.Duration
}

func dbPoolFor(cfg *config.Config) dbPoolConfig {
	if cfg != nil && cfg.DBType == string(SQLite) {
		if cfg.DBWAL {
			return dbPoolConfig{
				maxOpenConns: sqliteWALMaxOpenConns,
				maxIdleConns: sqliteWALMaxIdleConns,
			}
		}
		return dbPoolConfig{maxOpenConns: 1, maxIdleConns: 1}
	}
	return dbPoolConfig{
		maxOpenConns:    rdbmsMaxOpenConns,
		maxIdleConns:    rdbmsMaxIdleConns,
		connMaxLifetime: rdbmsConnMaxLifetime,
		connMaxIdleTime: rdbmsConnMaxIdleTime,
	}
}

func configureDBPool(db *gorm.DB, cfg *config.Config) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	applyDBPool(sqlDB, dbPoolFor(cfg))
	return nil
}

// configureDBPoolForTurso 配置 Turso 远程数据库的连接池。
// Turso 远程延迟高于本地文件，适当放宽连接数与生命周期。
func configureDBPoolForTurso(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(10 * time.Minute)
	sqlDB.SetConnMaxIdleTime(3 * time.Minute)
	return nil
}

func applyDBPool(sqlDB *sql.DB, pool dbPoolConfig) {
	sqlDB.SetMaxOpenConns(pool.maxOpenConns)
	sqlDB.SetMaxIdleConns(pool.maxIdleConns)
	sqlDB.SetConnMaxLifetime(pool.connMaxLifetime)
	sqlDB.SetConnMaxIdleTime(pool.connMaxIdleTime)
}

func parentDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return ""
}

// migrateStorageConfigSchema 修复 storage_configs 上不安全的 GORM default。
// 旧模型把 allowed_types / path_prefix 写成 gorm default，逗号与斜杠会被
// glebarez migrator 截断，生成非法 CREATE TABLE ...__temp SQL 并 panic。
// 这里在 AutoMigrate 前把表重建为无危险 default 的干净结构，并保留已有数据。
