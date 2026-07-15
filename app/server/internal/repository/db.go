package repository

import (
	"GOSpeak/internal/config"
	"GOSpeak/internal/model"
	"fmt"
	"os"
	"strings"

	"gorm.io/gorm"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
)

type DatabaseEnum string

const (
	PostgreSQL DatabaseEnum = "PostgreSQL"
	SQLite     DatabaseEnum = "SQLite"
	MySQL      DatabaseEnum = "MySQL"
)

var DB *gorm.DB

func InitDB(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	dbType := DatabaseEnum(cfg.DBType)
	var err error
	switch dbType {
	case PostgreSQL:
		DB, err = connectPostgreSQL(cfg)
	case SQLite:
		DB, err = connectSQLite(cfg)
	case MySQL:
		DB, err = connectMySQL(cfg)
	default:
		return fmt.Errorf("unsupported database type: %s", dbType)
	}
	if err != nil {
		panic(fmt.Sprintf("数据库连接失败 [%s]: %v", dbType, err))
	}

	sqlDB, err := DB.DB()
	if err != nil {
		panic(fmt.Sprintf("获取数据库实例失败 [%s]: %v", dbType, err))
	}
	if err = sqlDB.Ping(); err != nil {
		panic(fmt.Sprintf("数据库 Ping 失败 [%s]: %v", dbType, err))
	}

	if err := migrateOldSFUConfig(DB); err != nil {
		return err
	}
	if err := migrateStorageConfigSchema(DB); err != nil {
		return err
	}

	return autoMigrate()
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
	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}

func connectSQLite(cfg *config.Config) (*gorm.DB, error) {
	path := cfg.DBPath
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
	// WAL + busy_timeout 可降低热重载/并发写时的 SQLITE_BUSY。
	// MaxOpenConns(1) 避免同一进程多连接抢写锁。
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
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)
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
	return gorm.Open(mysql.Open(dsn), &gorm.Config{})
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
func migrateStorageConfigSchema(db *gorm.DB) error {
	if !db.Migrator().HasTable("storage_configs") {
		return nil
	}

	var ddl string
	if err := db.Raw(
		"SELECT sql FROM sqlite_master WHERE type = ? AND name = ?",
		"table", "storage_configs",
	).Scan(&ddl).Error; err != nil || ddl == "" {
		// 非 SQLite 或无法读取 sqlite_master：跳过，交给 AutoMigrate。
		return nil
	}

	// 旧 default 或非 gorm 风格 DDL（无反引号列名）都会让 glebarez migrator 解析失败。
	needsRebuild := strings.Contains(ddl, "image/jpeg,image/png") ||
		strings.Contains(ddl, `DEFAULT "uploads/"`) ||
		strings.Contains(ddl, "DEFAULT 'uploads/'") ||
		strings.Contains(ddl, "allowed_types text DEFAULT") ||
		strings.Contains(ddl, "`allowed_types` text DEFAULT") ||
		!strings.Contains(ddl, "`provider_type`")
	if !needsRebuild {
		return nil
	}

	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	// 使用 glebarez/gORM 常见的反引号单行 DDL，避免 migrator 解析多行/无引号列名失败。
	if err := tx.Exec("CREATE TABLE `storage_configs__rebuild` (`id` integer PRIMARY KEY AUTOINCREMENT,`provider_type` text,`endpoint` text,`bucket` text,`region` text,`access_key` text,`secret_key` text,`public_base_url` text,`path_prefix` text,`max_file_size` integer,`allowed_types` text,`created_at` datetime,`updated_at` datetime)").Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("migrate storage_configs schema: create rebuild table: %w", err)
	}
	if err := tx.Exec(`INSERT INTO storage_configs__rebuild (
	id, provider_type, endpoint, bucket, region, access_key, secret_key,
	public_base_url, path_prefix, max_file_size, allowed_types, created_at, updated_at
)
SELECT
	id, provider_type, endpoint, bucket, region, access_key, secret_key,
	public_base_url, path_prefix, max_file_size, allowed_types, created_at, updated_at
FROM storage_configs`).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("migrate storage_configs schema: copy data: %w", err)
	}
	if err := tx.Exec(`DROP TABLE storage_configs`).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("migrate storage_configs schema: drop old table: %w", err)
	}
	if err := tx.Exec(`ALTER TABLE storage_configs__rebuild RENAME TO storage_configs`).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("migrate storage_configs schema: rename rebuild table: %w", err)
	}
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("migrate storage_configs schema: commit: %w", err)
	}
	return nil
}

// migrateOldSFUConfig handles migration from the old single-row sfu_configs
// table (id as PK) to the new per-provider schema (provider as PK).
// Detection uses PRAGMA table_info (SQLite) to check the primary key column.
func migrateOldSFUConfig(db *gorm.DB) error {
	if !db.Migrator().HasTable("sfu_configs") {
		return nil
	}
	var pkColumn string
	if err := db.Raw("SELECT name FROM pragma_table_info(?) WHERE pk = 1", "sfu_configs").Scan(&pkColumn).Error; err != nil {
		return nil
	}
	if pkColumn != "id" {
		return nil
	}
	return db.Migrator().DropTable("sfu_configs")
}

func autoMigrate() error {
	return DB.AutoMigrate(
		&model.Role{},
		&model.User{},
		&model.Room{},
		&model.UserGroup{},
		&model.OAuthProvider{},
		&model.OAuthAccount{},
		&model.EmailConfig{},
		&model.EmailVerificationCode{},
		&model.SFUConfig{},
		&model.SFUActiveProvider{},
		&model.Permission{},
		&model.RolePermission{},
		&model.Mute{},
		&model.StorageConfig{},
		&model.BotToken{},
	)
}
