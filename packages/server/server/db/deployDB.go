package db

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
)

// DeploySQLite 初始化数据库连接，并自动创建文件和表
func DeploySQLite(dbPath string) error {
	var err error
	var _dbPath string
	if dbPath == "" {
		_dbPath = "./app.db"
	} else {
		_dbPath = dbPath
	}

	// 如果 dbPath 不存在，会自动创建该文件
	DB, err = gorm.Open(sqlite.Open(_dbPath), &gorm.Config{})
	if err != nil {
		return err
	}

	log.Printf("✅ 成功连接到数据库：%s", dbPath)

	// 🚀 自动迁移模式 —— 创建表（如果表已存在，则不会破坏数据）
	err = migrateSQLite()
	if err != nil {
		return err
	}

	log.Println("📊 数据库表迁移完成")
	return nil
}

// migrateSQLite 是私有函数，用于迁移数据表
func migrateSQLite() error {
	//type User struct {
	//	ID   uint
	//	Name string
	//}
	//
	//type Product struct {
	//	ID    uint
	//	Title string
	//}
	//
	//// 自动创建或更新表结构
	//err := DB.AutoMigrate(&User{}, &Product{})
	//if err != nil {
	//	return err
	//}
	//
	return nil
}

func init() {
	log.Println("Init DB....")
	err := DeploySQLite("./app.db")
	if err != nil {
		panic(err)
	}
}
