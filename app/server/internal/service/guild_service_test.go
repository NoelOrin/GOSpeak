package service

import (
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupGuildServiceTestDB(t *testing.T) (*GuildService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Guild{}, &model.GuildMember{}, &model.Room{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := repository.NewGuildRepository(db)
	svc := NewGuildService(repo)
	return svc, db
}

func seedGuildOwner(t *testing.T, db *gorm.DB, name, ownerUUID string) *model.Guild {
	t.Helper()
	g := &model.Guild{Name: name, OwnerUUID: ownerUUID}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed guild: %v", err)
	}
	m := &model.GuildMember{GuildUUID: g.UUID, UserUUID: ownerUUID, RoleName: GuildRoleOwner}
	if err := db.Create(m).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	return g
}

func TestGuildService_Create_EmptyName(t *testing.T) {
	svc, _ := setupGuildServiceTestDB(t)
	_, err := svc.Create("", "desc", "owner-uuid", false)
	checkAppErrCode(t, err, pkg.INVALID_PARAMS)
}

func TestGuildService_Create_NameTooLong(t *testing.T) {
	svc, _ := setupGuildServiceTestDB(t)
	longName := ""
	for i := 0; i < 101; i++ {
		longName += "x"
	}
	_, err := svc.Create(longName, "desc", "owner-uuid", false)
	checkAppErrCode(t, err, pkg.INVALID_PARAMS)
}

func TestGuildService_Create_Success(t *testing.T) {
	svc, db := setupGuildServiceTestDB(t)
	guild, err := svc.Create("Test Guild", "My server", "owner-uuid", true)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if guild.Name != "Test Guild" {
		t.Fatalf("expected name 'Test Guild', got %q", guild.Name)
	}
	if guild.OwnerUUID != "owner-uuid" {
		t.Fatalf("expected owner 'owner-uuid', got %q", guild.OwnerUUID)
	}
	if !guild.IsPublic {
		t.Fatal("expected guild to be public")
	}

	var member model.GuildMember
	if err := db.Where("guild_uuid = ? AND user_uuid = ?", guild.UUID, "owner-uuid").First(&member).Error; err != nil {
		t.Fatalf("owner member not found: %v", err)
	}
	if member.RoleName != GuildRoleOwner {
		t.Fatalf("expected owner role, got %q", member.RoleName)
	}
}

func TestGuildService_GetByUUID_NotFound(t *testing.T) {
	svc, _ := setupGuildServiceTestDB(t)
	_, err := svc.GetByUUID("nonexistent")
	checkAppErrCode(t, err, pkg.NOT_FOUND)
}

func TestGuildService_GetByUUID_Success(t *testing.T) {
	svc, db := setupGuildServiceTestDB(t)
	g := seedGuildOwner(t, db, "Test", "owner-1")

	got, err := svc.GetByUUID(g.UUID)
	if err != nil {
		t.Fatalf("GetByUUID error: %v", err)
	}
	if got.Name != "Test" {
		t.Fatalf("expected 'Test', got %q", got.Name)
	}
}

func TestGuildService_GetByInviteCode_NotFound(t *testing.T) {
	svc, _ := setupGuildServiceTestDB(t)
	_, err := svc.GetByInviteCode("INVALID")
	checkAppErrCode(t, err, pkg.NOT_FOUND)
}

func TestGuildService_Join_AlreadyMember(t *testing.T) {
	svc, db := setupGuildServiceTestDB(t)
	g := seedGuildOwner(t, db, "Test", "owner-1")

	_, err := svc.Join(g.InviteCode, "owner-1")
	checkAppErrCode(t, err, pkg.ALREADY_EXISTS)
}

func TestGuildService_Join_Success(t *testing.T) {
	svc, db := setupGuildServiceTestDB(t)
	g := seedGuildOwner(t, db, "Test", "owner-1")

	guild, err := svc.Join(g.InviteCode, "user-new")
	if err != nil {
		t.Fatalf("Join error: %v", err)
	}
	if guild.UUID != g.UUID {
		t.Fatalf("expected same guild UUID")
	}

	var member model.GuildMember
	if err := db.Where("guild_uuid = ? AND user_uuid = ?", g.UUID, "user-new").First(&member).Error; err != nil {
		t.Fatalf("member not found: %v", err)
	}
	if member.RoleName != GuildRoleMember {
		t.Fatalf("expected member role, got %q", member.RoleName)
	}
}

func TestGuildService_Join_InvalidCode(t *testing.T) {
	svc, _ := setupGuildServiceTestDB(t)
	_, err := svc.Join("BADCODE", "user-new")
	checkAppErrCode(t, err, pkg.NOT_FOUND)
}

func TestGuildService_Leave_OwnerCannotLeave(t *testing.T) {
	svc, db := setupGuildServiceTestDB(t)
	g := seedGuildOwner(t, db, "Test", "owner-1")

	err := svc.Leave(g.UUID, "owner-1")
	checkAppErrCode(t, err, pkg.FORBIDDEN)
}

func TestGuildService_Leave_Success(t *testing.T) {
	svc, db := setupGuildServiceTestDB(t)
	g := seedGuildOwner(t, db, "Test", "owner-1")

	repo := repository.NewGuildRepository(db)
	repo.AddMember(&model.GuildMember{GuildUUID: g.UUID, UserUUID: "member-1", RoleName: GuildRoleMember})

	if err := svc.Leave(g.UUID, "member-1"); err != nil {
		t.Fatalf("Leave error: %v", err)
	}

	_, err := repo.GetMember(g.UUID, "member-1")
	if err == nil {
		t.Fatal("expected member to be removed")
	}
}

func TestGuildService_Kick_Owner(t *testing.T) {
	svc, db := setupGuildServiceTestDB(t)
	g := seedGuildOwner(t, db, "Test", "owner-1")

	err := svc.Kick(g.UUID, "owner-1")
	checkAppErrCode(t, err, pkg.FORBIDDEN)
}

func TestGuildService_Kick_Success(t *testing.T) {
	svc, db := setupGuildServiceTestDB(t)
	g := seedGuildOwner(t, db, "Test", "owner-1")

	repo := repository.NewGuildRepository(db)
	repo.AddMember(&model.GuildMember{GuildUUID: g.UUID, UserUUID: "member-1", RoleName: GuildRoleMember})

	if err := svc.Kick(g.UUID, "member-1"); err != nil {
		t.Fatalf("Kick error: %v", err)
	}

	_, err := repo.GetMember(g.UUID, "member-1")
	if err == nil {
		t.Fatal("expected member to be removed after kick")
	}
}

func TestGuildService_TransferOwnership_NotOwner(t *testing.T) {
	svc, db := setupGuildServiceTestDB(t)
	g := seedGuildOwner(t, db, "Test", "owner-1")

	err := svc.TransferOwnership(g.UUID, "not-owner", "new-owner")
	checkAppErrCode(t, err, pkg.FORBIDDEN)
}

func TestGuildService_TransferOwnership_Success(t *testing.T) {
	svc, db := setupGuildServiceTestDB(t)
	g := seedGuildOwner(t, db, "Test", "owner-1")

	repo := repository.NewGuildRepository(db)
	repo.AddMember(&model.GuildMember{GuildUUID: g.UUID, UserUUID: "new-owner", RoleName: GuildRoleMember})

	if err := svc.TransferOwnership(g.UUID, "owner-1", "new-owner"); err != nil {
		t.Fatalf("TransferOwnership error: %v", err)
	}

	updatedGuild, err := repo.GetByUUID(g.UUID)
	if err != nil {
		t.Fatalf("get guild: %v", err)
	}
	if updatedGuild.OwnerUUID != "new-owner" {
		t.Fatalf("expected new owner, got %q", updatedGuild.OwnerUUID)
	}

	oldOwner, err := repo.GetMember(g.UUID, "owner-1")
	if err != nil {
		t.Fatalf("get old owner: %v", err)
	}
	if oldOwner.RoleName != GuildRoleAdmin {
		t.Fatalf("expected old owner to become admin, got %q", oldOwner.RoleName)
	}

	newOwner, err := repo.GetMember(g.UUID, "new-owner")
	if err != nil {
		t.Fatalf("get new owner: %v", err)
	}
	if newOwner.RoleName != GuildRoleOwner {
		t.Fatalf("expected new owner role, got %q", newOwner.RoleName)
	}
}

func TestGuildService_HasGuildRole(t *testing.T) {
	svc, db := setupGuildServiceTestDB(t)
	g := seedGuildOwner(t, db, "Test", "owner-1")

	repo := repository.NewGuildRepository(db)
	repo.AddMember(&model.GuildMember{GuildUUID: g.UUID, UserUUID: "admin-1", RoleName: GuildRoleAdmin})
	repo.AddMember(&model.GuildMember{GuildUUID: g.UUID, UserUUID: "member-1", RoleName: GuildRoleMember})
	repo.AddMember(&model.GuildMember{GuildUUID: g.UUID, UserUUID: "guest-1", RoleName: GuildRoleGuest})

	tests := []struct {
		userUUID string
		minRole  string
		want     bool
	}{
		{"owner-1", GuildRoleOwner, true},
		{"owner-1", GuildRoleAdmin, true},
		{"owner-1", GuildRoleGuest, true},
		{"admin-1", GuildRoleAdmin, true},
		{"admin-1", GuildRoleOwner, false},
		{"member-1", GuildRoleMember, true},
		{"member-1", GuildRoleAdmin, false},
		{"guest-1", GuildRoleGuest, true},
		{"guest-1", GuildRoleMember, false},
		{"nonexistent", GuildRoleGuest, false},
	}

	for _, tt := range tests {
		got := svc.HasGuildRole(g.UUID, tt.userUUID, tt.minRole)
		if got != tt.want {
			t.Errorf("HasGuildRole(%q, %q) = %v, want %v", tt.userUUID, tt.minRole, got, tt.want)
		}
	}
}

func TestGuildService_CheckRoomLimit_NoLimit(t *testing.T) {
	svc, db := setupGuildServiceTestDB(t)
	g := seedGuildOwner(t, db, "Test", "owner-1")

	if err := svc.CheckRoomLimit(g.UUID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGuildService_CheckRoomLimit_Reached(t *testing.T) {
	svc, db := setupGuildServiceTestDB(t)
	repo := repository.NewGuildRepository(db)

	g := seedGuildOwner(t, db, "Test", "owner-1")
	g.MaxRooms = 1
	if err := repo.Update(g); err != nil {
		t.Fatalf("update guild: %v", err)
	}

	r := &model.Room{Name: "room-1", GuildUUID: g.UUID}
	if err := db.Create(r).Error; err != nil {
		t.Fatalf("create room: %v", err)
	}

	err := svc.CheckRoomLimit(g.UUID)
	if err == nil {
		t.Fatal("expected error for room limit reached")
	}
	if err != ErrGuildRoomLimit {
		t.Fatalf("expected ErrGuildRoomLimit, got %v", err)
	}
}

func checkAppErrCode(t *testing.T, err error, code pkg.ErrCode) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := err.(*pkg.AppError)
	if !ok {
		t.Fatalf("expected *pkg.AppError, got %T", err)
	}
	if ae.Code != code {
		t.Fatalf("expected code=%d, got code=%d (msg=%q)", code, ae.Code, ae.Message)
	}
}
