package testutil

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"GOSpeak/internal/model"
)

// CreateTestUser 在测试数据库中创建一个用户。
func CreateTestUser(db *gorm.DB, name string) *model.User {
	user := &model.User{
		Name:     name,
		Password: "testpass",
		Role:     "user",
	}
	if err := db.Create(user).Error; err != nil {
		panic("CreateTestUser: " + err.Error())
	}
	return user
}

// CreateTestGuild 在测试数据库中创建一个 Guild。
func CreateTestGuild(db *gorm.DB, name, ownerUUID string) *model.Guild {
	guild := &model.Guild{
		Name:      name,
		OwnerUUID: ownerUUID,
	}
	if err := db.Create(guild).Error; err != nil {
		panic("CreateTestGuild: " + err.Error())
	}
	return guild
}

// AddTestGuildMember 添加 Guild 成员。
func AddTestGuildMember(db *gorm.DB, guildUUID, userUUID, role string) *model.GuildMember {
	member := &model.GuildMember{
		GuildUUID: guildUUID,
		UserUUID:  userUUID,
		RoleName:  role,
	}
	if err := db.Create(member).Error; err != nil {
		panic("AddTestGuildMember: " + err.Error())
	}
	return member
}

// SetupTestDB 初始化内存 SQLite 并迁移指定的模型。
func SetupTestDB(t interface{ Fatalf(string, ...interface{}) }, models ...interface{}) *gorm.DB {
	_ = t
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("SetupTestDB open: " + err.Error())
	}
	if err := db.AutoMigrate(models...); err != nil {
		panic("SetupTestDB migrate: " + err.Error())
	}
	return db
}
