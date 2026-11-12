package repository

import (
	"testing"

	"GOSpeak/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// seedLegacyGuestSchema 建出「访客体系还在时」的旧库结构：
// domains/users 带 guest 列与 invite_code，users.is_guest 与 domains.invite_code 带索引，
// 另有一张 domain_guest_bans 表。
func seedLegacyGuestSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	stmts := []string{
		`CREATE TABLE domains (
			id integer primary key autoincrement, uuid text, name text not null,
			icon_url text, description text, owner_uuid uuid not null, invite_code text,
			is_public numeric default false, allow_guest numeric default false,
			guest_can_listen numeric default true, guest_can_speak numeric default true,
			guest_can_message numeric default false, guest_limit integer default 50,
			created_at datetime, updated_at datetime)`,
		`CREATE UNIQUE INDEX idx_domains_uuid ON domains(uuid)`,
		`CREATE INDEX idx_domains_owner_uuid ON domains(owner_uuid)`,
		`CREATE UNIQUE INDEX idx_domains_invite_code ON domains(invite_code)`,
		`CREATE TABLE users (
			id integer primary key autoincrement, uuid text, name text, display_name text,
			avatar text, email text, email_verified numeric default false,
			is_bot numeric default false, is_guest numeric default false, password text,
			role text default 'user', status text default 'active', token_version integer default 0,
			created_at datetime, updated_at datetime)`,
		`CREATE UNIQUE INDEX idx_users_uuid ON users(uuid)`,
		`CREATE UNIQUE INDEX idx_users_name ON users(name)`,
		`CREATE INDEX idx_users_email ON users(email)`,
		`CREATE INDEX idx_users_is_guest ON users(is_guest)`,
		`CREATE INDEX idx_users_status ON users(status)`,
		`CREATE TABLE domain_guest_bans (
			id integer primary key autoincrement, domain_uuid text, user_uuid text,
			reason text, banned_by text, expires_at datetime, created_at datetime, updated_at datetime)`,
		`INSERT INTO domains (uuid,name,owner_uuid,invite_code,is_public) VALUES ('d1','Legacy','o1','ABCDEFGH',0)`,
		`INSERT INTO users (uuid,name,is_guest) VALUES ('u1','alice',0)`,
		`INSERT INTO domain_guest_bans (domain_uuid,user_uuid) VALUES ('d1','u1')`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("seed legacy schema (%s): %v", stmt, err)
		}
	}
}

// TestDropLegacyGuestSchema_RemovesColumnsAndTable 覆盖真实升级路径：
// 老库（列与索引都在）→ 删列删表 → 数据保留 → 二次执行幂等。
func TestDropLegacyGuestSchema_RemovesColumnsAndTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	seedLegacyGuestSchema(t, db)

	if err := dropLegacyGuestSchema(db); err != nil {
		t.Fatalf("dropLegacyGuestSchema: %v", err)
	}

	for _, col := range []string{"allow_guest", "guest_can_listen", "guest_can_speak", "guest_can_message", "guest_limit", "invite_code"} {
		if db.Migrator().HasColumn("domains", col) {
			t.Fatalf("domains.%s must be dropped", col)
		}
	}
	if db.Migrator().HasColumn("users", "is_guest") {
		t.Fatal("users.is_guest must be dropped")
	}
	if db.Migrator().HasTable("domain_guest_bans") {
		t.Fatal("domain_guest_bans must be dropped")
	}

	// 存量数据必须保留。
	var name, owner string
	if err := db.Raw(`SELECT name, owner_uuid FROM domains WHERE uuid = 'd1'`).Row().Scan(&name, &owner); err != nil {
		t.Fatalf("read surviving domain row: %v", err)
	}
	if name != "Legacy" || owner != "o1" {
		t.Fatalf("domain row corrupted: name=%q owner=%q", name, owner)
	}
	var userName string
	if err := db.Raw(`SELECT name FROM users WHERE uuid = 'u1'`).Row().Scan(&userName); err != nil {
		t.Fatalf("read surviving user row: %v", err)
	}
	if userName != "alice" {
		t.Fatalf("user row corrupted: name=%q", userName)
	}

	// 幂等：再跑一次不报错。
	if err := dropLegacyGuestSchema(db); err != nil {
		t.Fatalf("second dropLegacyGuestSchema must be a no-op: %v", err)
	}
}

// TestDropLegacyGuestSchema_OnFreshDatabase 新库（无旧列）必须静默通过。
func TestDropLegacyGuestSchema_OnFreshDatabase(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Domain{}, &model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := dropLegacyGuestSchema(db); err != nil {
		t.Fatalf("dropLegacyGuestSchema on fresh db: %v", err)
	}
}

// TestDropLegacyGuestSchema_NoTablesAtAll 空库（连表都没有）也必须静默通过。
func TestDropLegacyGuestSchema_NoTablesAtAll(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := dropLegacyGuestSchema(db); err != nil {
		t.Fatalf("dropLegacyGuestSchema on empty db: %v", err)
	}
}
