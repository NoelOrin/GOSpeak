package repository

import (
	"fmt"
	"go_rtc/internal/model"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type DatabaseEnum string

const (
	PostgreSQL DatabaseEnum = "PostgreSQL"
	SQLite     DatabaseEnum = "SQLite"
	MySQL      DatabaseEnum = "MySQL"
)

var DB *gorm.DB

func InitDB() error {
	dbType := getDBType()
	var err error
	switch dbType {
	case PostgreSQL:
		DB, err = connectPostgreSQL()
	case SQLite:
		DB, err = connectSQLite()
	case MySQL:
		DB, err = connectMySQL()
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

	return autoMigrate()
}

func getDBType() DatabaseEnum {
	dbType := os.Getenv("DB_TYPE")
	if dbType == "" {
		return SQLite
	}
	return DatabaseEnum(dbType)
}

func connectPostgreSQL() (*gorm.DB, error) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = fmt.Sprintf(
			"host=%s user=%s password=%s dbname=myapp port=%s sslmode=disable",
			getEnv("DB_HOST", "localhost"),
			getEnv("DB_USER", "postgres"),
			getEnv("DB_PASSWORD", "postgres"),
			getEnv("DB_PORT", "5432"),
		)
	}
	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}

func connectSQLite() (*gorm.DB, error) {
	path := os.Getenv("DB_PATH")
	if path == "" {
		if err := os.MkdirAll("db", 0755); err != nil {
			return nil, fmt.Errorf("failed to create db directory: %w", err)
		}
		path = "db/app.db"
	}
	walEnabled := os.Getenv("DB_WAL") == "true"
	journalMode := "DELETE"
	if walEnabled {
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
	if walEnabled {
		sqlDB.Exec("PRAGMA synchronous=NORMAL")
	} else {
		sqlDB.Exec("PRAGMA synchronous=FULL")
	}
	sqlDB.Exec("PRAGMA foreign_keys=ON")
	return db, nil
}

func connectMySQL() (*gorm.DB, error) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/myapp?charset=utf8mb4&parseTime=True&loc=Local",
			getEnv("DB_USER", "root"),
			getEnv("DB_PASSWORD", ""),
			getEnv("DB_HOST", "localhost"),
			getEnv("DB_PORT", "3306"),
		)
	}
	return gorm.Open(mysql.Open(dsn), &gorm.Config{})
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func autoMigrate() error {
	return DB.AutoMigrate(
		&model.Role{},
		&model.User{},
		&model.Room{},
		&model.UserGroup{},
		&model.OAuthProvider{},
		&model.OAuthAccount{},
		&model.Permission{},
		&model.RolePermission{},
	)
}
