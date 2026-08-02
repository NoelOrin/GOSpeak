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

// DomainOption 是 CreateTestDomain 的功能选项。
type DomainOption func(g *model.Domain)

// WithDomainInviteCode 设置邀请码。
func WithDomainInviteCode(code string) DomainOption {
	return func(g *model.Domain) { g.InviteCode = code }
}

// CreateTestDomain 在测试数据库中创建一个 Domain。
func CreateTestDomain(db *gorm.DB, name, ownerUUID string, opts ...DomainOption) *model.Domain {
	domain := &model.Domain{
		Name:      name,
		OwnerUUID: ownerUUID,
	}
	for _, o := range opts {
		o(domain)
	}
	if err := db.Create(domain).Error; err != nil {
		panic("CreateTestDomain: " + err.Error())
	}
	return domain
}

// AddTestDomainMember 添加 Domain 成员。
func AddTestDomainMember(db *gorm.DB, domainUUID, userUUID, role string) *model.DomainMember {
	member := &model.DomainMember{
		DomainUUID: domainUUID,
		UserUUID:  userUUID,
		RoleName:  role,
	}
	if err := db.Create(member).Error; err != nil {
		panic("AddTestDomainMember: " + err.Error())
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
