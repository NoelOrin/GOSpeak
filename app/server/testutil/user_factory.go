package testutil

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"GOSpeak/internal/model"
)

// UserOption 是 CreateTestUser 的功能选项。
type UserOption func(u *model.User)

// WithPassword 设置用户密码。
func WithPassword(pwd string) UserOption {
	return func(u *model.User) { u.Password = pwd }
}

// WithRole 设置用户角色。
func WithRole(role string) UserOption {
	return func(u *model.User) { u.Role = role }
}

// WithUserUUID 设置用户 UUID（自动生成时不需要）。
func WithUserUUID(uuid string) UserOption {
	return func(u *model.User) { u.UUID = uuid }
}

// WithDisplayName 设置显示名称。
func WithDisplayName(name string) UserOption {
	return func(u *model.User) { u.DisplayName = name }
}

// CreateTestUser 在测试数据库中创建一个用户。
func CreateTestUser(db *gorm.DB, name string, opts ...UserOption) *model.User {
	user := &model.User{
		Name:     name,
		Password: "testpass",
		Role:     "user",
	}
	for _, o := range opts {
		o(user)
	}
	if err := db.Create(user).Error; err != nil {
		panic("CreateTestUser: " + err.Error())
	}
	return user
}

// GuildOption 是 CreateTestGuild 的功能选项。
type GuildOption func(g *model.Guild)

// WithGuildInviteCode 设置邀请码。
func WithGuildInviteCode(code string) GuildOption {
	return func(g *model.Guild) { g.InviteCode = code }
}

// WithGuildMaxRooms 设置房间上限。
func WithGuildMaxRooms(limit int) GuildOption {
	return func(g *model.Guild) { g.MaxRooms = uint(limit) }
}

// CreateTestGuild 在测试数据库中创建一个 Guild。
func CreateTestGuild(db *gorm.DB, name, ownerUUID string, opts ...GuildOption) *model.Guild {
	guild := &model.Guild{
		Name:      name,
		OwnerUUID: ownerUUID,
	}
	for _, o := range opts {
		o(guild)
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
