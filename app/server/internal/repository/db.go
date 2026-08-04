package repository

import (
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"GOSpeak/internal/config"
	"GOSpeak/internal/model"
)

type DatabaseEnum string

const (
	PostgreSQL DatabaseEnum = "PostgreSQL"
	SQLite     DatabaseEnum = "SQLite"
	MySQL      DatabaseEnum = "MySQL"
)

var (
	dbMu sync.RWMutex
	DB   *gorm.DB
)

// setDB atomically writes the global DB handle.

// setDB atomically writes the global DB handle.
func setDB(db *gorm.DB) {
	dbMu.Lock()
	DB = db
	dbMu.Unlock()
}

// getDB returns the global DB handle.

// getDB returns the global DB handle.
func getDB() *gorm.DB {
	dbMu.RLock()
	defer dbMu.RUnlock()
	return DB
}

func InitDB(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
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
		return fmt.Errorf("unsupported database type: %s", dbType)
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

	if cfg.IsWorker() {
		// Worker 数据面不负责 schema 迁移，避免对共享权威库产生写操作。
		return nil
	}

	if err := migrateOldSFUConfig(db); err != nil {
		return err
	}
	if err := migrateStorageConfigSchema(DB); err != nil {
		return err
	}
	if err := migrateBotTokensSchema(DB); err != nil {
		return err
	}
	if err := migrateEmailVerificationCodesSchema(DB); err != nil {
		return err
	}

	if err := autoMigrate(); err != nil {
		return err
	}

	if err := migrateDomainCUID2(DB); err != nil {
		return err
	}

	if err := migrateRoomPasswords(DB); err != nil {
		return err
	}

	return nil
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
	path := cfg.DBPath
	if cfg.ClusterRole == model.ClusterRoleWorker {
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
	if path == "" {
		if err := os.MkdirAll("db", 0755); err != nil {
			return nil, fmt.Errorf("failed to create db directory: %w", err)
		}
		path = "db/app.db"
	} else if dir := parentDir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create db directory: %w", err)
		}
	}
	// glebarez/modernc 使用 _pragma=...，旧的 _journal_mode/_busy_timeout 不会生效。
	// WAL + busy_timeout 可降低热重载/并发写时的 SQLITE_BUSY，并支持多连接并发读。
	// 非 WAL 时连接池会强制 MaxOpenConns=1，避免 DELETE 日志模式写锁竞争。
	journalMode := "DELETE"
	if cfg.DBWAL {
		journalMode = "WAL"
	}
	dsn := fmt.Sprintf(
		"%s?_pragma=busy_timeout(10000)&_pragma=journal_mode(%s)&_pragma=foreign_keys(1)",
		path, journalMode,
	)
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
