package repository

import (
	"GOSpeak/internal/config"
	"GOSpeak/internal/model"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

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

var (
	dbMu sync.RWMutex
	DB   *gorm.DB
)

// setDB atomically writes the global DB handle.
func setDB(db *gorm.DB) {
	dbMu.Lock()
	DB = db
	dbMu.Unlock()
}

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

	if err := migrateDefaultGuild(DB); err != nil {
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
const (
	sqliteWALMaxOpenConns = 4
	sqliteWALMaxIdleConns = 4

	rdbmsMaxOpenConns = 25
	rdbmsMaxIdleConns = 10
)

var (
	rdbmsConnMaxLifetime  = 30 * time.Minute
	rdbmsConnMaxIdleTime = 5 * time.Minute
)

type dbPoolConfig struct {
	maxOpenConns    int
	maxIdleConns    int
	connMaxLifetime  time.Duration
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
		connMaxLifetime:  rdbmsConnMaxLifetime,
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

// migrateBotTokensSchema 修复 bot_tokens DDL 中的 glebarez/modernec 不兼容问题：
// 1) `permissions_json TEXT` 无反引号列名
// 2) `DEFAULT "user"` 双引号字面量（modernc 会误解析为标识符）
func migrateBotTokensSchema(db *gorm.DB) error {
	if !db.Migrator().HasTable("bot_tokens") {
		return nil
	}
	var ddl string
	if err := db.Raw(
		"SELECT sql FROM sqlite_master WHERE type = ? AND name = ?",
		"table", "bot_tokens",
	).Scan(&ddl).Error; err != nil || ddl == "" {
		return nil
	}
	// 无反引号列名 或 DEFAULT "xxx" 双引号字面量
	if !strings.Contains(ddl, "permissions_json TEXT") && !strings.Contains(ddl, `DEFAULT "user"`) {
		return nil
	}

	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	if err := tx.Exec(`CREATE TABLE "bot_tokens__rebuild" (` +
		"`id` integer PRIMARY KEY AUTOINCREMENT," +
		"`uuid` uuid," +
		"`name` text," +
		"`user_uuid` uuid," +
		"`role` text DEFAULT 'user'," +
		"`revoked` numeric DEFAULT false," +
		"`permissions` text," +
		"`expires_at` datetime," +
		"`created_at` datetime," +
		"`updated_at` datetime," +
		"`created_by` text," +
		"`last_used_at` datetime" +
		`)`).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("migrate bot_tokens: create rebuild table: %w", err)
	}
	if err := tx.Exec(`INSERT INTO "bot_tokens__rebuild" (
		id, uuid, name, user_uuid, role, revoked, permissions,
		expires_at, created_at, updated_at, created_by, last_used_at
	) SELECT
		id, uuid, name, user_uuid, role, revoked, permissions,
		expires_at, created_at, updated_at, created_by, last_used_at
	FROM bot_tokens`).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("migrate bot_tokens: copy data: %w", err)
	}
	if err := tx.Exec(`DROP TABLE bot_tokens`).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("migrate bot_tokens: drop old table: %w", err)
	}
	if err := tx.Exec(`ALTER TABLE "bot_tokens__rebuild" RENAME TO bot_tokens`).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("migrate bot_tokens: rename rebuild table: %w", err)
	}
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("migrate bot_tokens: commit: %w", err)
	}
	return nil
}

// migrateEmailVerificationCodesSchema 清理 email_verification_codes 上历史残留的
// 无反引号索引 DDL（glebarez 解析多行/无反引号索引定义时会 panic）。
func migrateEmailVerificationCodesSchema(db *gorm.DB) error {
	if !db.Migrator().HasTable("email_verification_codes") {
		return nil
	}
	// 只查 SQLite 风格索引（glebarez 仅用于 SQLite）
	if db.Migrator().HasIndex("email_verification_codes", "idx_email_codes_email_scene_created") {
		if err := db.Migrator().DropIndex("email_verification_codes", "idx_email_codes_email_scene_created"); err != nil {
			// fallback: raw SQL
			if err2 := db.Exec(`DROP INDEX IF EXISTS idx_email_codes_email_scene_created`).Error; err2 != nil {
				return fmt.Errorf("migrate email_verification_codes: drop legacy index: %w", err2)
			}
		}
	}
	return nil
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
		&model.PluginConfig{},
		&model.Message{},
		&model.MessageReaction{},
		&model.MessageMention{},
		&model.Guild{},
		&model.GuildMember{},
	)
}

// migrateDefaultGuild 创建默认 Guild 并将无归属的存量房间归入其中。
// 仅在没有任何 Guild 存在时执行，确保存量数据平滑迁移到多 Servers 架构。
func migrateDefaultGuild(db *gorm.DB) error {
	var count int64
	db.Model(&model.Guild{}).Count(&count)
	if count > 0 {
		return nil
	}
	defaultGuild := &model.Guild{
		Name: "Default Server", Description: "系统默认语音服务器", IsPublic: false,
	}
	if err := db.Create(defaultGuild).Error; err != nil {
		return err
	}
	// 将现有无 guild_uuid 的房间归入默认 Guild
	return db.Model(&model.Room{}).Where("guild_uuid = ?", "").
		Update("guild_uuid", defaultGuild.UUID).Error
}
