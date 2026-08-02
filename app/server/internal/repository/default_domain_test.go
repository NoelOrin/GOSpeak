package repository

import (
	"strings"
	"testing"

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

func TestMigrateDomainCUID2(t *testing.T) {
	db := newDomainTestDB(t)
	const legacyUUID = "11111111-2222-3333-4444-555555555555"
	domain := &model.Domain{Name: "legacy", UUID: legacyUUID}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := db.Create(&model.DomainMember{DomainUUID: legacyUUID, UserUUID: "user-1", RoleName: "owner"}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	if err := db.Create(&model.Room{Name: "room-1", DomainUUID: legacyUUID}).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}

	if err := migrateDomainCUID2(db); err != nil {
		t.Fatalf("migrateDomainCUID2: %v", err)
	}

	var updated model.Domain
	if err := db.Table("domains").Select("uuid").Scan(&updated.UUID).Error; err != nil {
		t.Fatalf("load domain uuid: %v", err)
	}
	if strings.Contains(updated.UUID, "-") {
		t.Fatalf("expected cuid2 without dashes, got %q", updated.UUID)
	}
	if updated.UUID == legacyUUID {
		t.Fatal("expected uuid to be regenerated")
	}
	assertColumnCount(t, db, "domain_members", "domain_uuid", updated.UUID, 1)
	assertColumnCount(t, db, "room", "domain_uuid", updated.UUID, 1)
	assertColumnCount(t, db, "domain_members", "domain_uuid", legacyUUID, 0)
	assertColumnCount(t, db, "room", "domain_uuid", legacyUUID, 0)

	// 幂等：第二次运行不应改写已迁移的 cuid2。
	before := updated.UUID
	if err := migrateDomainCUID2(db); err != nil {
		t.Fatalf("second migrateDomainCUID2: %v", err)
	}
	if err := db.Table("domains").Select("uuid").Scan(&updated.UUID).Error; err != nil {
		t.Fatalf("reload domain uuid: %v", err)
	}
	if updated.UUID != before {
		t.Fatalf("expected idempotent migration, got %q", updated.UUID)
	}
}
