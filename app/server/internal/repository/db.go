package repository

import (
	"GOSpeak/internal/config"
	"GOSpeak/internal/model"
	"fmt"
	"os"

	"gorm.io/gorm"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"github.com/glebarez/sqlite"
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
	journalMode := "DELETE"
	if cfg.DBWAL {
		journalMode = "WAL"
	}
	dsn := path + fmt.Sprintf("?_journal_mode=%s&_busy_timeout=5000", journalMode)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.Exec(fmt.Sprintf("PRAGMA journal_mode=%s", journalMode))
	if cfg.DBWAL {
		sqlDB.Exec("PRAGMA synchronous=NORMAL")
	} else {
		sqlDB.Exec("PRAGMA synchronous=FULL")
	}
	sqlDB.Exec("PRAGMA foreign_keys=ON")
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
