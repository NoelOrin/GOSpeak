package service

import (
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupDomainServiceTestDB(t *testing.T) (*DomainService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Domain{}, &model.DomainMember{}, &model.Room{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := repository.NewDomainRepository(db)
	svc := NewDomainService(repo)
	return svc, db
}

func seedDomainOwner(t *testing.T, db *gorm.DB, name, ownerUUID string) *model.Domain {
	t.Helper()
	g := &model.Domain{Name: name, OwnerUUID: ownerUUID}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	m := &model.DomainMember{DomainUUID: g.UUID, UserUUID: ownerUUID, RoleName: DomainRoleOwner}
	if err := db.Create(m).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	return g
}

func TestDomainService_Create_EmptyName(t *testing.T) {
	svc, _ := setupDomainServiceTestDB(t)
	_, err := svc.Create("", "desc", "owner-uuid", false)
	checkAppErrCode(t, err, pkg.INVALID_PARAMS)
}

func TestDomainService_Create_NameTooLong(t *testing.T) {
	svc, _ := setupDomainServiceTestDB(t)
	longName := ""
	for i := 0; i < 101; i++ {
		longName += "x"
	}
	_, err := svc.Create(longName, "desc", "owner-uuid", false)
	checkAppErrCode(t, err, pkg.INVALID_PARAMS)
}

func TestDomainService_Create_Success(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	domain, err := svc.Create("Test Domain", "My server", "owner-uuid", true)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if domain.Name != "Test Domain" {
		t.Fatalf("expected name 'Test Domain', got %q", domain.Name)
	}
	if domain.OwnerUUID != "owner-uuid" {
		t.Fatalf("expected owner 'owner-uuid', got %q", domain.OwnerUUID)
	}
	if !domain.IsPublic {
		t.Fatal("expected domain to be public")
	}

	var member model.DomainMember
	if err := db.Where("domain_uuid = ? AND user_uuid = ?", domain.UUID, "owner-uuid").First(&member).Error; err != nil {
		t.Fatalf("owner member not found: %v", err)
	}
	if member.RoleName != DomainRoleOwner {
		t.Fatalf("expected owner role, got %q", member.RoleName)
	}
}

func TestDomainService_GetByUUID_NotFound(t *testing.T) {
	svc, _ := setupDomainServiceTestDB(t)
	_, err := svc.GetByUUID("nonexistent")
	checkAppErrCode(t, err, pkg.NOT_FOUND)
}

func TestDomainService_GetByUUID_Success(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	g := seedDomainOwner(t, db, "Test", "owner-1")

	got, err := svc.GetByUUID(g.UUID)
	if err != nil {
		t.Fatalf("GetByUUID error: %v", err)
	}
	if got.Name != "Test" {
		t.Fatalf("expected 'Test', got %q", got.Name)
	}
}

func TestDomainService_GetByInviteCode_NotFound(t *testing.T) {
	svc, _ := setupDomainServiceTestDB(t)
	_, err := svc.GetByInviteCode("INVALID")
	checkAppErrCode(t, err, pkg.NOT_FOUND)
}

func TestDomainService_Join_AlreadyMember(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	g := seedDomainOwner(t, db, "Test", "owner-1")

	_, err := svc.Join(g.InviteCode, "owner-1")
	checkAppErrCode(t, err, pkg.ALREADY_EXISTS)
}

func TestDomainService_Join_Success(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	g := seedDomainOwner(t, db, "Test", "owner-1")

	domain, err := svc.Join(g.InviteCode, "user-new")
	if err != nil {
		t.Fatalf("Join error: %v", err)
	}
	if domain.UUID != g.UUID {
		t.Fatalf("expected same domain UUID")
	}

	var member model.DomainMember
	if err := db.Where("domain_uuid = ? AND user_uuid = ?", g.UUID, "user-new").First(&member).Error; err != nil {
		t.Fatalf("member not found: %v", err)
	}
	if member.RoleName != DomainRoleMember {
		t.Fatalf("expected member role, got %q", member.RoleName)
	}
}

func TestDomainService_Join_InvalidCode(t *testing.T) {
	svc, _ := setupDomainServiceTestDB(t)
	_, err := svc.Join("BADCODE", "user-new")
	checkAppErrCode(t, err, pkg.NOT_FOUND)
}

func TestDomainService_Leave_OwnerCannotLeave(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	g := seedDomainOwner(t, db, "Test", "owner-1")

	err := svc.Leave(g.UUID, "owner-1")
	checkAppErrCode(t, err, pkg.FORBIDDEN)
}

func TestDomainService_Leave_Success(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	g := seedDomainOwner(t, db, "Test", "owner-1")

	repo := repository.NewDomainRepository(db)
	repo.AddMember(&model.DomainMember{DomainUUID: g.UUID, UserUUID: "member-1", RoleName: DomainRoleMember})

	if err := svc.Leave(g.UUID, "member-1"); err != nil {
		t.Fatalf("Leave error: %v", err)
	}

	_, err := repo.GetMember(g.UUID, "member-1")
	if err == nil {
		t.Fatal("expected member to be removed")
	}
}

func TestDomainService_Kick_Owner(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	g := seedDomainOwner(t, db, "Test", "owner-1")

	err := svc.Kick(g.UUID, "owner-1")
	checkAppErrCode(t, err, pkg.FORBIDDEN)
}

func TestDomainService_Kick_Success(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	g := seedDomainOwner(t, db, "Test", "owner-1")

	repo := repository.NewDomainRepository(db)
	repo.AddMember(&model.DomainMember{DomainUUID: g.UUID, UserUUID: "member-1", RoleName: DomainRoleMember})

	if err := svc.Kick(g.UUID, "member-1"); err != nil {
		t.Fatalf("Kick error: %v", err)
	}

	_, err := repo.GetMember(g.UUID, "member-1")
	if err == nil {
		t.Fatal("expected member to be removed after kick")
	}
}

func TestDomainService_TransferOwnership_NotOwner(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	g := seedDomainOwner(t, db, "Test", "owner-1")

	err := svc.TransferOwnership(g.UUID, "not-owner", "new-owner")
	checkAppErrCode(t, err, pkg.FORBIDDEN)
}

func TestDomainService_TransferOwnership_Success(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	g := seedDomainOwner(t, db, "Test", "owner-1")

	repo := repository.NewDomainRepository(db)
	repo.AddMember(&model.DomainMember{DomainUUID: g.UUID, UserUUID: "new-owner", RoleName: DomainRoleMember})

	if err := svc.TransferOwnership(g.UUID, "owner-1", "new-owner"); err != nil {
		t.Fatalf("TransferOwnership error: %v", err)
	}

	updatedDomain, err := repo.GetByUUID(g.UUID)
	if err != nil {
		t.Fatalf("get domain: %v", err)
	}
	if updatedDomain.OwnerUUID != "new-owner" {
		t.Fatalf("expected new owner, got %q", updatedDomain.OwnerUUID)
	}

	oldOwner, err := repo.GetMember(g.UUID, "owner-1")
	if err != nil {
		t.Fatalf("get old owner: %v", err)
	}
	if oldOwner.RoleName != DomainRoleAdmin {
		t.Fatalf("expected old owner to become admin, got %q", oldOwner.RoleName)
	}

	newOwner, err := repo.GetMember(g.UUID, "new-owner")
	if err != nil {
		t.Fatalf("get new owner: %v", err)
	}
	if newOwner.RoleName != DomainRoleOwner {
		t.Fatalf("expected new owner role, got %q", newOwner.RoleName)
	}
}

func TestDomainService_HasDomainRole(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	g := seedDomainOwner(t, db, "Test", "owner-1")

	repo := repository.NewDomainRepository(db)
	repo.AddMember(&model.DomainMember{DomainUUID: g.UUID, UserUUID: "admin-1", RoleName: DomainRoleAdmin})
	repo.AddMember(&model.DomainMember{DomainUUID: g.UUID, UserUUID: "member-1", RoleName: DomainRoleMember})
	repo.AddMember(&model.DomainMember{DomainUUID: g.UUID, UserUUID: "guest-1", RoleName: DomainRoleGuest})

	tests := []struct {
		userUUID string
		minRole  string
		want     bool
	}{
		{"owner-1", DomainRoleOwner, true},
		{"owner-1", DomainRoleAdmin, true},
		{"owner-1", DomainRoleGuest, true},
		{"admin-1", DomainRoleAdmin, true},
		{"admin-1", DomainRoleOwner, false},
		{"member-1", DomainRoleMember, true},
		{"member-1", DomainRoleAdmin, false},
		{"guest-1", DomainRoleGuest, true},
		{"guest-1", DomainRoleMember, false},
		{"nonexistent", DomainRoleGuest, false},
	}

	for _, tt := range tests {
		got := svc.HasDomainRole(g.UUID, tt.userUUID, tt.minRole)
		if got != tt.want {
			t.Errorf("HasDomainRole(%q, %q) = %v, want %v", tt.userUUID, tt.minRole, got, tt.want)
		}
	}
}

func TestDomainService_CheckRoomLimit_NoLimit(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	g := seedDomainOwner(t, db, "Test", "owner-1")

	if err := svc.CheckRoomLimit(g.UUID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDomainService_CheckRoomLimit_Reached(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	repo := repository.NewDomainRepository(db)

	g := seedDomainOwner(t, db, "Test", "owner-1")
	g.MaxRooms = 1
	if err := repo.Update(g); err != nil {
		t.Fatalf("update domain: %v", err)
	}

	r := &model.Room{Name: "room-1", DomainUUID: g.UUID}
	if err := db.Create(r).Error; err != nil {
		t.Fatalf("create room: %v", err)
	}

	err := svc.CheckRoomLimit(g.UUID)
	if err == nil {
		t.Fatal("expected error for room limit reached")
	}
	if err != ErrDomainRoomLimit {
		t.Fatalf("expected ErrDomainRoomLimit, got %v", err)
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
