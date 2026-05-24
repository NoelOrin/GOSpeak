package db

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"os"
)

type DatabaseEnum string

const (
	PostgreSQL DatabaseEnum = "PostgreSQL"
	SQLite     DatabaseEnum = "SQLite"
	MySQL      DatabaseEnum = "MySQL"
)

var DB *gorm.DB // 全局 DB 变量

func ConnectDB() error {
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
		return fmt.Errorf("不支持的数据库类型: %s", dbType)
	}

	if err != nil {
		return fmt.Errorf("连接 %s 数据库失败: %w", dbType, err)
	}

	return nil
}

func getDBType() DatabaseEnum {
	dbType := os.Getenv("DB_TYPE")
	if dbType == "" {
		return SQLite // 默认使用 SQLite
	}
	return DatabaseEnum(dbType)
}

// 各数据库连接函数
func connectPostgreSQL() (*gorm.DB, error) {
	dsn := os.Getenv("DB_DSN") // 例如: "host=localhost user=postgres password=123456 dbname=myapp port=5432 sslmode=disable"
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
		path = "app.db" // 默认数据库文件
	}
	return gorm.Open(sqlite.Open(path), &gorm.Config{})
}

func connectMySQL() (*gorm.DB, error) {
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
	return gorm.Open(mysql.Open(dsn), &gorm.Config{})

}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func init() {
	err := ConnectDB()
	if err != nil {
		panic(err)
	}
}
