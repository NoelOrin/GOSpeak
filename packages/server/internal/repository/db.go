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
		return fmt.Errorf("failed to connect %s: %w", dbType, err)
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
		path = "app.db"
	}
	return gorm.Open(sqlite.Open(path), &gorm.Config{})
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
		&model.User{},
		&model.Room{},
		&model.UserGroup{},
	)
}