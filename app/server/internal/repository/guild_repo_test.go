package repository

import (
	"testing"

	"GOSpeak/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newGuildTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Guild{}, &model.GuildMember{}, &model.Room{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestGuildRepo_Create(t *testing.T) {
	db := newGuildTestDB(t)
	repo := NewGuildRepository(db)

	guild := &model.Guild{Name: "Test Guild", OwnerUUID: "owner-uuid-1"}
	if err := repo.Create(guild); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if guild.UUID == "" {
		t.Fatal("expected UUID to be set after create")
	}
	if guild.InviteCode == "" {
		t.Fatal("expected InviteCode to be set after create")
	}

	var saved model.Guild
	if err := db.First(&saved, "uuid = ?", guild.UUID).Error; err != nil {
		t.Fatalf("guild not found in DB: %v", err)
	}
	if saved.Name != "Test Guild" {
		t.Fatalf("expected name 'Test Guild', got %q", saved.Name)
	}
}

func TestGuildRepo_GetByUUID(t *testing.T) {
	db := newGuildTestDB(t)
	repo := NewGuildRepository(db)

	guild := &model.Guild{Name: "Test", OwnerUUID: "owner-1"}
	if err := repo.Create(guild); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	got, err := repo.GetByUUID(guild.UUID)
	if err != nil {
		t.Fatalf("GetByUUID error: %v", err)
	}
	if got.Name != "Test" {
		t.Fatalf("expected name 'Test', got %q", got.Name)
	}
}

func TestGuildRepo_GetByInviteCode(t *testing.T) {
	db := newGuildTestDB(t)
	repo := NewGuildRepository(db)

	guild := &model.Guild{Name: "Test", OwnerUUID: "owner-1"}
	if err := repo.Create(guild); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	got, err := repo.GetByInviteCode(guild.InviteCode)
	if err != nil {
		t.Fatalf("GetByInviteCode error: %v", err)
	}
	if got.Name != "Test" {
		t.Fatalf("expected name 'Test', got %q", got.Name)
	}
}

func TestGuildRepo_List(t *testing.T) {
	db := newGuildTestDB(t)
	repo := NewGuildRepository(db)

	// explicit invite codes to avoid collision in rapid creation
	for i, name := range []string{"Guild-A", "Guild-B", "Guild-C", "Guild-D", "Guild-E"} {
		prefix := string(rune('A' + i))
		code := prefix + prefix + "ABCDEF"
		g := &model.Guild{
			Name:       name,
			OwnerUUID:  "owner-1",
			InviteCode: code,
		}
		if err := repo.Create(g); err != nil {
			t.Fatalf("Create error: %v", err)
		}
	}

	guilds, total, err := repo.List(1, 3)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(guilds) != 3 {
		t.Fatalf("expected 3 guilds, got %d", len(guilds))
	}
	if total != 5 {
		t.Fatalf("expected total=5, got %d", total)
	}

	// page 2
	guilds2, _, err := repo.List(2, 3)
	if err != nil {
		t.Fatalf("List page2 error: %v", err)
	}
	if len(guilds2) != 2 {
		t.Fatalf("expected 2 guilds on page2, got %d", len(guilds2))
	}
}

func TestGuildRepo_ListPublic(t *testing.T) {
	db := newGuildTestDB(t)
	repo := NewGuildRepository(db)

	for i := 0; i < 3; i++ {
		g := &model.Guild{
			Name:      "Public-" + string(rune('A'+i)),
			OwnerUUID: "owner-1",
			IsPublic:  true,
		}
		if err := repo.Create(g); err != nil {
			t.Fatalf("Create error: %v", err)
		}
	}
	// private guild
	g := &model.Guild{Name: "Private", OwnerUUID: "owner-1", IsPublic: false}
	if err := repo.Create(g); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	guilds, total, err := repo.ListPublic(1, 10, "")
	if err != nil {
		t.Fatalf("ListPublic error: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total=3, got %d", total)
	}
	if len(guilds) != 3 {
		t.Fatalf("expected 3 guilds, got %d", len(guilds))
	}
}

func TestGuildRepo_AddMember(t *testing.T) {
	db := newGuildTestDB(t)
	repo := NewGuildRepository(db)

	guild := &model.Guild{Name: "Test", OwnerUUID: "owner-1"}
	if err := repo.Create(guild); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	member := &model.GuildMember{
		GuildUUID: guild.UUID,
		UserUUID:  "user-1",
		RoleName:  "member",
	}
	if err := repo.AddMember(member); err != nil {
		t.Fatalf("AddMember error: %v", err)
	}

	var saved model.GuildMember
	if err := db.Where("guild_uuid = ? AND user_uuid = ?", guild.UUID, "user-1").First(&saved).Error; err != nil {
		t.Fatalf("member not found: %v", err)
	}
	if saved.RoleName != "member" {
		t.Fatalf("expected role 'member', got %q", saved.RoleName)
	}
}

func TestGuildRepo_RemoveMember(t *testing.T) {
	db := newGuildTestDB(t)
	repo := NewGuildRepository(db)

	guild := &model.Guild{Name: "Test", OwnerUUID: "owner-1"}
	if err := repo.Create(guild); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	member := &model.GuildMember{GuildUUID: guild.UUID, UserUUID: "user-1"}
	if err := repo.AddMember(member); err != nil {
		t.Fatalf("AddMember error: %v", err)
	}

	if err := repo.RemoveMember(guild.UUID, "user-1"); err != nil {
		t.Fatalf("RemoveMember error: %v", err)
	}

	var count int64
	db.Model(&model.GuildMember{}).Where("guild_uuid = ? AND user_uuid = ?", guild.UUID, "user-1").Count(&count)
	if count != 0 {
		t.Fatal("expected no rows after remove")
	}
}

func TestGuildRepo_GetMember(t *testing.T) {
	db := newGuildTestDB(t)
	repo := NewGuildRepository(db)

	guild := &model.Guild{Name: "Test", OwnerUUID: "owner-1"}
	if err := repo.Create(guild); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	member := &model.GuildMember{GuildUUID: guild.UUID, UserUUID: "user-1", RoleName: "admin"}
	if err := repo.AddMember(member); err != nil {
		t.Fatalf("AddMember error: %v", err)
	}

	got, err := repo.GetMember(guild.UUID, "user-1")
	if err != nil {
		t.Fatalf("GetMember error: %v", err)
	}
	if got.RoleName != "admin" {
		t.Fatalf("expected role 'admin', got %q", got.RoleName)
	}

	_, err = repo.GetMember(guild.UUID, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent member")
	}
}

func TestGuildRepo_ListMembers(t *testing.T) {
	db := newGuildTestDB(t)
	repo := NewGuildRepository(db)

	guild := &model.Guild{Name: "Test", OwnerUUID: "owner-1"}
	if err := repo.Create(guild); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	for i := 0; i < 3; i++ {
		uid := string(rune('A' + i))
		member := &model.GuildMember{GuildUUID: guild.UUID, UserUUID: uid, RoleName: "member"}
		if err := repo.AddMember(member); err != nil {
			t.Fatalf("AddMember error: %v", err)
		}
	}

	members, err := repo.ListMembers(guild.UUID)
	if err != nil {
		t.Fatalf("ListMembers error: %v", err)
	}
	if len(members) != 3 {
		t.Fatalf("expected 3 members, got %d", len(members))
	}
}

func TestGuildRepo_ListUserGuilds(t *testing.T) {
	db := newGuildTestDB(t)
	repo := NewGuildRepository(db)

	g1 := &model.Guild{Name: "G1", OwnerUUID: "o1"}
	if err := repo.Create(g1); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	g2 := &model.Guild{Name: "G2", OwnerUUID: "o2"}
	if err := repo.Create(g2); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	repo.AddMember(&model.GuildMember{GuildUUID: g1.UUID, UserUUID: "user-x"})
	repo.AddMember(&model.GuildMember{GuildUUID: g2.UUID, UserUUID: "user-x"})

	uuids, err := repo.ListUserGuilds("user-x")
	if err != nil {
		t.Fatalf("ListUserGuilds error: %v", err)
	}
	if len(uuids) != 2 {
		t.Fatalf("expected 2 guild UUIDs, got %d", len(uuids))
	}
}

func TestGuildRepo_CountMembers(t *testing.T) {
	db := newGuildTestDB(t)
	repo := NewGuildRepository(db)

	guild := &model.Guild{Name: "Test", OwnerUUID: "owner-1"}
	if err := repo.Create(guild); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	for i := 0; i < 4; i++ {
		uid := string(rune('A' + i))
		repo.AddMember(&model.GuildMember{GuildUUID: guild.UUID, UserUUID: uid})
	}

	count, err := repo.CountMembers(guild.UUID)
	if err != nil {
		t.Fatalf("CountMembers error: %v", err)
	}
	if count != 4 {
		t.Fatalf("expected 4 members, got %d", count)
	}
}

func TestGuildRepo_CountRooms(t *testing.T) {
	db := newGuildTestDB(t)
	repo := NewGuildRepository(db)

	guild := &model.Guild{Name: "Test", OwnerUUID: "owner-1"}
	if err := repo.Create(guild); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	for i := 0; i < 3; i++ {
		room := &model.Room{
			Name:      "room-" + string(rune('A'+i)),
			GuildUUID: guild.UUID,
		}
		if err := db.Create(room).Error; err != nil {
			t.Fatalf("Create room error: %v", err)
		}
	}

	count, err := repo.CountRooms(guild.UUID)
	if err != nil {
		t.Fatalf("CountRooms error: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 rooms, got %d", count)
	}
}

func TestGuildRepo_Delete(t *testing.T) {
	db := newGuildTestDB(t)
	repo := NewGuildRepository(db)

	guild := &model.Guild{Name: "Test", OwnerUUID: "owner-1"}
	if err := repo.Create(guild); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	if err := repo.Delete(guild.UUID); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	var count int64
	db.Model(&model.Guild{}).Where("uuid = ?", guild.UUID).Count(&count)
	if count != 0 {
		t.Fatal("expected no rows after delete")
	}
}

func TestGuildRepo_Delete_NotFound(t *testing.T) {
	db := newGuildTestDB(t)
	repo := NewGuildRepository(db)

	// Deleting a non-existent UUID should not error (GORM returns nil for zero rows)
	if err := repo.Delete("nonexistent-uuid"); err != nil {
		t.Fatalf("Delete nonexistent error: %v", err)
	}
}

func TestGuildRepo_Delete_CleansMembersAndRooms(t *testing.T) {
	db := newGuildTestDB(t)
	repo := NewGuildRepository(db)

	guild := &model.Guild{Name: "Test", OwnerUUID: "owner-1"}
	if err := repo.Create(guild); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if err := repo.AddMember(&model.GuildMember{GuildUUID: guild.UUID, UserUUID: "member-1"}); err != nil {
		t.Fatalf("AddMember error: %v", err)
	}
	if err := db.Create(&model.Room{Name: "room-1", GuildUUID: guild.UUID}).Error; err != nil {
		t.Fatalf("Create room error: %v", err)
	}

	if err := repo.Delete(guild.UUID); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	var count int64
	db.Model(&model.Guild{}).Where("uuid = ?", guild.UUID).Count(&count)
	if count != 0 {
		t.Fatalf("expected guild removed, count=%d", count)
	}
	db.Model(&model.GuildMember{}).Where("guild_uuid = ?", guild.UUID).Count(&count)
	if count != 0 {
		t.Fatalf("expected members removed, count=%d", count)
	}
	db.Model(&model.Room{}).Where("guild_uuid = ?", guild.UUID).Count(&count)
	if count != 0 {
		t.Fatalf("expected rooms removed, count=%d", count)
	}
}
