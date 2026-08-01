package repository

import (
	"testing"

	"GOSpeak/internal/model"
)

func TestMigrateDefaultGuild_CreatesOwnerAndMember(t *testing.T) {
	db := newGuildTestDB(t)
	if err := db.Create(&model.Room{Name: "legacy", GuildUUID: ""}).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}

	if err := EnsureDefaultGuild(db, "admin-uuid"); err != nil {
		t.Fatalf("EnsureDefaultGuild: %v", err)
	}

	var guild model.Guild
	if err := db.Where("name = ?", "Default Server").First(&guild).Error; err != nil {
		t.Fatalf("default guild not found: %v", err)
	}
	if guild.OwnerUUID != "admin-uuid" {
		t.Fatalf("expected owner admin-uuid, got %q", guild.OwnerUUID)
	}

	var member model.GuildMember
	if err := db.Where("guild_uuid = ? AND user_uuid = ?", guild.UUID, "admin-uuid").First(&member).Error; err != nil {
		t.Fatalf("owner member not found: %v", err)
	}
	if member.RoleName != "owner" {
		t.Fatalf("expected owner role, got %q", member.RoleName)
	}

	var room model.Room
	if err := db.Where("name = ?", "legacy").First(&room).Error; err != nil {
		t.Fatalf("legacy room not found: %v", err)
	}
	if room.GuildUUID != guild.UUID {
		t.Fatalf("expected legacy room moved to default guild, got %q", room.GuildUUID)
	}
}

func TestMigrateDefaultGuild_RepairsMissingOwner(t *testing.T) {
	db := newGuildTestDB(t)
	broken := &model.Guild{Name: "Default Server", Description: "legacy default", OwnerUUID: ""}
	if err := db.Create(broken).Error; err != nil {
		t.Fatalf("seed broken guild: %v", err)
	}
	if err := db.Create(&model.Room{Name: "legacy", GuildUUID: ""}).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}

	if err := EnsureDefaultGuild(db, "admin-uuid"); err != nil {
		t.Fatalf("EnsureDefaultGuild: %v", err)
	}

	var guild model.Guild
	if err := db.Where("uuid = ?", broken.UUID).First(&guild).Error; err != nil {
		t.Fatalf("default guild not found: %v", err)
	}
	if guild.OwnerUUID != "admin-uuid" {
		t.Fatalf("expected owner repaired, got %q", guild.OwnerUUID)
	}

	var member model.GuildMember
	if err := db.Where("guild_uuid = ? AND user_uuid = ?", guild.UUID, "admin-uuid").First(&member).Error; err != nil {
		t.Fatalf("owner member not found: %v", err)
	}
	if member.RoleName != "owner" {
		t.Fatalf("expected owner role, got %q", member.RoleName)
	}

	var room model.Room
	if err := db.Where("name = ?", "legacy").First(&room).Error; err != nil {
		t.Fatalf("legacy room not found: %v", err)
	}
	if room.GuildUUID != "" {
		t.Fatalf("existing default guild must not migrate platform rooms, got %q", room.GuildUUID)
	}
}

func TestMigrateDefaultGuild_RepairsMissingOwnerMember(t *testing.T) {
	db := newGuildTestDB(t)
	broken := &model.Guild{Name: "Default Server", Description: "legacy default", OwnerUUID: "existing-owner"}
	if err := db.Create(broken).Error; err != nil {
		t.Fatalf("seed broken guild: %v", err)
	}

	if err := EnsureDefaultGuild(db, "admin-uuid"); err != nil {
		t.Fatalf("EnsureDefaultGuild: %v", err)
	}

	var member model.GuildMember
	if err := db.Where("guild_uuid = ? AND user_uuid = ?", broken.UUID, "existing-owner").First(&member).Error; err != nil {
		t.Fatalf("owner member not found: %v", err)
	}
	if member.RoleName != "owner" {
		t.Fatalf("expected owner role, got %q", member.RoleName)
	}
}

func TestMigrateDefaultGuild_ExistingDefaultDoesNotMigratePlatformRooms(t *testing.T) {
	db := newGuildTestDB(t)
	if err := EnsureDefaultGuild(db, "admin-uuid"); err != nil {
		t.Fatalf("EnsureDefaultGuild: %v", err)
	}
	if err := db.Create(&model.Room{Name: "platform", GuildUUID: ""}).Error; err != nil {
		t.Fatalf("seed platform room: %v", err)
	}

	if err := EnsureDefaultGuild(db, "admin-uuid"); err != nil {
		t.Fatalf("second EnsureDefaultGuild: %v", err)
	}

	var room model.Room
	if err := db.Where("name = ?", "platform").First(&room).Error; err != nil {
		t.Fatalf("platform room not found: %v", err)
	}
	if room.GuildUUID != "" {
		t.Fatalf("expected platform room to remain unassigned, got %q", room.GuildUUID)
	}
}
