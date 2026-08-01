package repository

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"GOSpeak/internal/model"
)

func TestMigrateDefaultDomain_CreatesOwnerAndMember(t *testing.T) {
	db := newDomainTestDB(t)
	if err := db.Create(&model.Room{Name: "legacy", DomainUUID: ""}).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}

	if err := EnsureDefaultDomain(db, "admin-uuid"); err != nil {
		t.Fatalf("EnsureDefaultDomain: %v", err)
	}

	var domain model.Domain
	if err := db.Where("name = ?", "默认域").First(&domain).Error; err != nil {
		t.Fatalf("default domain not found: %v", err)
	}
	if domain.OwnerUUID != "admin-uuid" {
		t.Fatalf("expected owner admin-uuid, got %q", domain.OwnerUUID)
	}

	var member model.DomainMember
	if err := db.Where("domain_uuid = ? AND user_uuid = ?", domain.UUID, "admin-uuid").First(&member).Error; err != nil {
		t.Fatalf("owner member not found: %v", err)
	}
	if member.RoleName != "owner" {
		t.Fatalf("expected owner role, got %q", member.RoleName)
	}

	var room model.Room
	if err := db.Where("name = ?", "legacy").First(&room).Error; err != nil {
		t.Fatalf("legacy room not found: %v", err)
	}
	if room.DomainUUID != domain.UUID {
		t.Fatalf("expected legacy room moved to default domain, got %q", room.DomainUUID)
	}
}

func TestMigrateDefaultDomain_RepairsMissingOwner(t *testing.T) {
	db := newDomainTestDB(t)
	broken := &model.Domain{Name: "默认域", Description: "legacy default", OwnerUUID: ""}
	if err := db.Create(broken).Error; err != nil {
		t.Fatalf("seed broken domain: %v", err)
	}
	if err := db.Create(&model.Room{Name: "legacy", DomainUUID: ""}).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}

	if err := EnsureDefaultDomain(db, "admin-uuid"); err != nil {
		t.Fatalf("EnsureDefaultDomain: %v", err)
	}

	var domain model.Domain
	if err := db.Where("uuid = ?", broken.UUID).First(&domain).Error; err != nil {
		t.Fatalf("default domain not found: %v", err)
	}
	if domain.OwnerUUID != "admin-uuid" {
		t.Fatalf("expected owner repaired, got %q", domain.OwnerUUID)
	}

	var member model.DomainMember
	if err := db.Where("domain_uuid = ? AND user_uuid = ?", domain.UUID, "admin-uuid").First(&member).Error; err != nil {
		t.Fatalf("owner member not found: %v", err)
	}
	if member.RoleName != "owner" {
		t.Fatalf("expected owner role, got %q", member.RoleName)
	}

	var room model.Room
	if err := db.Where("name = ?", "legacy").First(&room).Error; err != nil {
		t.Fatalf("legacy room not found: %v", err)
	}
	if room.DomainUUID != "" {
		t.Fatalf("existing default domain must not migrate platform rooms, got %q", room.DomainUUID)
	}
}

func TestMigrateDefaultDomain_RepairsMissingOwnerMember(t *testing.T) {
	db := newDomainTestDB(t)
	broken := &model.Domain{Name: "默认域", Description: "legacy default", OwnerUUID: "existing-owner"}
	if err := db.Create(broken).Error; err != nil {
		t.Fatalf("seed broken domain: %v", err)
	}

	if err := EnsureDefaultDomain(db, "admin-uuid"); err != nil {
		t.Fatalf("EnsureDefaultDomain: %v", err)
	}

	var member model.DomainMember
	if err := db.Where("domain_uuid = ? AND user_uuid = ?", broken.UUID, "existing-owner").First(&member).Error; err != nil {
		t.Fatalf("owner member not found: %v", err)
	}
	if member.RoleName != "owner" {
		t.Fatalf("expected owner role, got %q", member.RoleName)
	}
}

func TestMigrateDefaultDomain_ExistingDefaultDoesNotMigratePlatformRooms(t *testing.T) {
	db := newDomainTestDB(t)
	if err := EnsureDefaultDomain(db, "admin-uuid"); err != nil {
		t.Fatalf("EnsureDefaultDomain: %v", err)
	}
	if err := db.Create(&model.Room{Name: "platform", DomainUUID: ""}).Error; err != nil {
		t.Fatalf("seed platform room: %v", err)
	}

	if err := EnsureDefaultDomain(db, "admin-uuid"); err != nil {
		t.Fatalf("second EnsureDefaultDomain: %v", err)
	}

	var room model.Room
	if err := db.Where("name = ?", "platform").First(&room).Error; err != nil {
		t.Fatalf("platform room not found: %v", err)
	}
	if room.DomainUUID != "" {
		t.Fatalf("expected platform room to remain unassigned, got %q", room.DomainUUID)
	}
}

func newLegacyGuildTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	stmts := []string{
		`CREATE TABLE guilds (id INTEGER PRIMARY KEY AUTOINCREMENT, uuid TEXT, name TEXT, description TEXT, owner_uuid TEXT, invite_code TEXT, max_rooms INTEGER, is_public INTEGER, created_at datetime, updated_at datetime)`,
		`CREATE TABLE guild_members (id INTEGER PRIMARY KEY AUTOINCREMENT, guild_uuid TEXT, user_uuid TEXT, nickname TEXT, role_name TEXT, joined_at datetime)`,
		`CREATE TABLE room (id INTEGER PRIMARY KEY AUTOINCREMENT, uuid TEXT, name TEXT, guild_uuid TEXT, created_at datetime, updated_at datetime)`,
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			t.Fatalf("create legacy table: %v", err)
		}
	}
	return db
}

func assertColumnCount(t *testing.T, db *gorm.DB, table, column, value string, want int64) {
	t.Helper()
	var got int64
	if err := db.Table(table).Where(column+" = ?", value).Count(&got).Error; err != nil {
		t.Fatalf("count %s.%s=%q: %v", table, column, value, err)
	}
	if got != want {
		t.Fatalf("expected %d rows in %s.%s=%q, got %d", want, table, column, value, got)
	}
}

func TestMigrateGuildToDomainSchema(t *testing.T) {
	db := newLegacyGuildTestDB(t)
	const legacyUUID = "11111111-2222-3333-4444-555555555555"
	if err := db.Exec(`INSERT INTO guilds (uuid, name, owner_uuid) VALUES (?, ?, ?)`, legacyUUID, "legacy", "admin-uuid").Error; err != nil {
		t.Fatalf("seed guild: %v", err)
	}
	if err := db.Exec(`INSERT INTO guild_members (guild_uuid, user_uuid, role_name) VALUES (?, ?, ?)`, legacyUUID, "user-1", "owner").Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	if err := db.Exec(`INSERT INTO room (uuid, name, guild_uuid) VALUES (?, ?, ?)`, "room-uuid-1", "room-1", legacyUUID).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}

	if err := migrateGuildToDomainSchema(db); err != nil {
		t.Fatalf("migrateGuildToDomainSchema: %v", err)
	}

	if !db.Migrator().HasTable("domains") || db.Migrator().HasTable("guilds") {
		t.Fatal("expected guilds renamed to domains")
	}
	if !db.Migrator().HasTable("domain_members") || db.Migrator().HasTable("guild_members") {
		t.Fatal("expected guild_members renamed to domain_members")
	}
	if !db.Migrator().HasColumn("domain_members", "domain_uuid") || db.Migrator().HasColumn("domain_members", "guild_uuid") {
		t.Fatal("expected guild_uuid renamed to domain_uuid in domain_members")
	}
	if !db.Migrator().HasColumn("room", "domain_uuid") || db.Migrator().HasColumn("room", "guild_uuid") {
		t.Fatal("expected room.guild_uuid renamed to domain_uuid")
	}

	var domain struct{ UUID string }
	if err := db.Table("domains").Select("uuid").Scan(&domain).Error; err != nil {
		t.Fatalf("load domain uuid: %v", err)
	}
	if domain.UUID != legacyUUID {
		t.Fatalf("expected legacy uuid preserved, got %q", domain.UUID)
	}
}

func TestMigrateDomainCUID2(t *testing.T) {
	db := newLegacyGuildTestDB(t)
	const legacyUUID = "11111111-2222-3333-4444-555555555555"
	if err := db.Exec(`INSERT INTO guilds (uuid, name, owner_uuid) VALUES (?, ?, ?)`, legacyUUID, "legacy", "admin-uuid").Error; err != nil {
		t.Fatalf("seed guild: %v", err)
	}
	if err := db.Exec(`INSERT INTO guild_members (guild_uuid, user_uuid, role_name) VALUES (?, ?, ?)`, legacyUUID, "user-1", "owner").Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	if err := db.Exec(`INSERT INTO room (uuid, name, guild_uuid) VALUES (?, ?, ?)`, "room-uuid-1", "room-1", legacyUUID).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}

	if err := migrateGuildToDomainSchema(db); err != nil {
		t.Fatalf("migrateGuildToDomainSchema: %v", err)
	}
	if err := migrateDomainCUID2(db); err != nil {
		t.Fatalf("migrateDomainCUID2: %v", err)
	}

	var domain struct{ UUID string }
	if err := db.Table("domains").Select("uuid").Scan(&domain).Error; err != nil {
		t.Fatalf("load domain uuid: %v", err)
	}
	if strings.Contains(domain.UUID, "-") {
		t.Fatalf("expected cuid2 without dashes, got %q", domain.UUID)
	}
	if domain.UUID == legacyUUID {
		t.Fatal("expected uuid to be regenerated")
	}
	assertColumnCount(t, db, "domain_members", "domain_uuid", domain.UUID, 1)
	assertColumnCount(t, db, "room", "domain_uuid", domain.UUID, 1)
	assertColumnCount(t, db, "domain_members", "domain_uuid", legacyUUID, 0)
	assertColumnCount(t, db, "room", "domain_uuid", legacyUUID, 0)

	// 幂等：第二次运行不应改写已迁移的 cuid2。
	before := domain.UUID
	if err := migrateDomainCUID2(db); err != nil {
		t.Fatalf("second migrateDomainCUID2: %v", err)
	}
	if err := db.Table("domains").Select("uuid").Scan(&domain).Error; err != nil {
		t.Fatalf("reload domain uuid: %v", err)
	}
	if domain.UUID != before {
		t.Fatalf("expected idempotent migration, got %q", domain.UUID)
	}
}

func TestMigrateGuildPermissions(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE permissions (id INTEGER PRIMARY KEY AUTOINCREMENT, code TEXT, name TEXT, description TEXT)`).Error; err != nil {
		t.Fatalf("create permissions: %v", err)
	}
	if err := db.Exec(`CREATE TABLE bot_tokens (id INTEGER PRIMARY KEY AUTOINCREMENT, permissions TEXT)`).Error; err != nil {
		t.Fatalf("create bot_tokens: %v", err)
	}
	rows := [][3]string{
		{"guild:create", "创建语音服务器", "允许创建语音服务器"},
		{"domain:manage", "管理域", "管理域"},
		{"room:create", "创建房间", "允许创建房间"},
	}
	for _, r := range rows {
		if err := db.Exec(`INSERT INTO permissions (code, name, description) VALUES (?, ?, ?)`, r[0], r[1], r[2]).Error; err != nil {
			t.Fatalf("seed permission: %v", err)
		}
	}
	if err := db.Exec(`INSERT INTO bot_tokens (permissions) VALUES (?)`, "guild:create,guild:manage").Error; err != nil {
		t.Fatalf("seed bot token: %v", err)
	}

	if err := migrateGuildPermissions(db); err != nil {
		t.Fatalf("migrateGuildPermissions: %v", err)
	}

	var perm struct {
		Code        string
		Name        string
		Description string
	}
	if err := db.Table("permissions").Where("code = ?", "domain:create").Scan(&perm).Error; err != nil {
		t.Fatalf("load migrated permission: %v", err)
	}
	if perm.Name != "创建域" || perm.Description != "允许创建域" {
		t.Fatalf("expected guild labels replaced, got name=%q description=%q", perm.Name, perm.Description)
	}
	if err := db.Table("permissions").Where("code = ?", "domain:manage").Scan(&perm).Error; err != nil {
		t.Fatalf("load domain:manage permission: %v", err)
	}
	if perm.Name != "管理域" {
		t.Fatalf("expected unchanged label, got %q", perm.Name)
	}
	if err := db.Table("permissions").Where("code = ?", "room:create").Scan(&perm).Error; err != nil {
		t.Fatalf("load room:create permission: %v", err)
	}
	if perm.Name != "创建房间" {
		t.Fatalf("expected room permission untouched, got %q", perm.Name)
	}

	var bt string
	if err := db.Table("bot_tokens").Select("permissions").Scan(&bt).Error; err != nil {
		t.Fatalf("load bot token: %v", err)
	}
	if bt != "domain:create,domain:manage" {
		t.Fatalf("expected bot permissions migrated, got %q", bt)
	}
}
