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
	if err := db.AutoMigrate(&model.Domain{}, &model.DomainMember{}, &model.Room{}, &model.DomainRole{}, &model.DomainRolePermission{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := repository.NewDomainRepository(db)
	roleRepo := repository.NewDomainRoleRepository(db)
	svc := NewDomainService(repo, roleRepo)
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

func TestDomainService_IsMemberUsesCacheUntilInvalidated(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	g := seedDomainOwner(t, db, "Test", "owner-1")

	if !svc.IsMember(g.UUID, "owner-1") {
		t.Fatal("owner should be a member")
	}

	repo := repository.NewDomainRepository(db)
	if err := repo.RemoveMember(g.UUID, "owner-1"); err != nil {
		t.Fatalf("remove member: %v", err)
	}

	if !svc.IsMember(g.UUID, "owner-1") {
		t.Fatal("expected cached membership to remain true")
	}

	svc.invalidateMembership(g.UUID, "owner-1", false)
	if svc.IsMember(g.UUID, "owner-1") {
		t.Fatal("expected invalidated membership to become false")
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

func TestDomainService_HasDomainPermission(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	domain := &model.Domain{Name: "Perm", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.DomainMember{
		DomainUUID: domain.UUID, UserUUID: "owner-1", RoleName: model.DomainRoleOwner,
	}).Error; err != nil {
		t.Fatalf("seed owner member: %v", err)
	}
	if err := db.Create(&model.DomainMember{
		DomainUUID: domain.UUID, UserUUID: "admin-1", RoleName: model.DomainRoleAdmin,
	}).Error; err != nil {
		t.Fatalf("seed admin member: %v", err)
	}
	if err := db.Create(&model.DomainMember{
		DomainUUID: domain.UUID, UserUUID: "member-1", RoleName: model.DomainRoleMember,
	}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	if !svc.HasDomainPermission(domain.UUID, "owner-1", model.PermRoomDelete) {
		t.Error("owner must have room:delete")
	}
	if !svc.HasDomainPermission(domain.UUID, "admin-1", model.PermDomainKick) {
		t.Error("admin must have domain:kick by default")
	}
	if svc.HasDomainPermission(domain.UUID, "member-1", model.PermRoomDelete) {
		t.Error("member must not have room:delete without explicit grant")
	}
}

func TestDomainService_AdminHasAllAssignablePermissions(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	domain := &model.Domain{Name: "Admin Perms", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.DomainMember{
		DomainUUID: domain.UUID, UserUUID: "admin-1", RoleName: model.DomainRoleAdmin,
	}).Error; err != nil {
		t.Fatalf("seed admin member: %v", err)
	}

	// 模拟存量库 admin 权限行缺失/不完整，admin 仍应默认拥有全部可分配域权限。
	if err := db.Where("domain_uuid = ? AND role_name = ?", domain.UUID, model.DomainRoleAdmin).
		Delete(&model.DomainRolePermission{}).Error; err != nil {
		t.Fatalf("clear admin rows: %v", err)
	}

	got, err := svc.GetDomainRolePermissions(domain.UUID, model.DomainRoleAdmin)
	if err != nil {
		t.Fatalf("GetDomainRolePermissions: %v", err)
	}
	if len(got) != len(model.AssignableDomainPermissions) {
		t.Fatalf("admin permissions = %d, want %d", len(got), len(model.AssignableDomainPermissions))
	}
	for _, code := range model.AssignableDomainPermissions {
		if !svc.HasDomainPermission(domain.UUID, "admin-1", code) {
			t.Errorf("admin must have %q by default", code)
		}
	}
}

func TestDomainService_AdminAndOwnerPermissionsAreFixed(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	domain := &model.Domain{Name: "Fixed", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}

	for _, roleName := range []string{model.DomainRoleOwner, model.DomainRoleAdmin} {
		if err := svc.UpdateDomainRolePermissions(domain.UUID, roleName, []string{model.PermRoomRead}); err == nil {
			t.Errorf("UpdateDomainRolePermissions(%q) must be forbidden", roleName)
		} else if ae, ok := err.(*pkg.AppError); !ok || ae.Code != pkg.FORBIDDEN {
			t.Errorf("UpdateDomainRolePermissions(%q) expected FORBIDDEN, got %v", roleName, err)
		}
	}
}

func TestDomainService_ListDomainRoles_IsolatedPerDomain(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	domainA := &model.Domain{Name: "A", OwnerUUID: "owner-1"}
	domainB := &model.Domain{Name: "B", OwnerUUID: "owner-1"}
	if err := db.Create(domainA).Error; err != nil {
		t.Fatalf("seed domain A: %v", err)
	}
	if err := db.Create(domainB).Error; err != nil {
		t.Fatalf("seed domain B: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domainA.UUID); err != nil {
		t.Fatalf("seed roles A: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domainB.UUID); err != nil {
		t.Fatalf("seed roles B: %v", err)
	}
	if err := svc.CreateDomainRole(domainA.UUID, "moderator", []string{model.PermRoomRead}); err != nil {
		t.Fatalf("create custom role in A: %v", err)
	}

	rolesB, err := svc.ListDomainRoles(domainB.UUID)
	if err != nil {
		t.Fatalf("ListDomainRoles(B): %v", err)
	}
	for _, role := range rolesB {
		if role.Name == "moderator" {
			t.Fatal("domain B must not see domain A's custom role")
		}
	}
}

func TestDomainService_CreateAndDeleteCustomRole(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	domain := &model.Domain{Name: "Custom", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := svc.CreateDomainRole(domain.UUID, "moderator", []string{model.PermRoomRead}); err != nil {
		t.Fatalf("CreateDomainRole: %v", err)
	}
	codes, err := svc.GetDomainRolePermissions(domain.UUID, "moderator")
	if err != nil {
		t.Fatalf("GetDomainRolePermissions: %v", err)
	}
	if len(codes) != 1 {
		t.Fatalf("expected 1 permission, got %v", codes)
	}
	if err := svc.CreateDomainRole(domain.UUID, "admin", nil); err == nil {
		t.Fatal("must reject creating system role name")
	}
	if err := svc.DeleteDomainRole(domain.UUID, "admin"); err == nil {
		t.Fatal("must reject deleting system role")
	}
	if err := svc.DeleteDomainRole(domain.UUID, "moderator"); err != nil {
		t.Fatalf("DeleteDomainRole: %v", err)
	}
}

func TestDomainService_SetMemberRole(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	domain := &model.Domain{Name: "Members", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.DomainMember{
		DomainUUID: domain.UUID, UserUUID: "u-2", RoleName: model.DomainRoleMember,
	}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	if err := svc.SetMemberRole(domain.UUID, "owner-1", "u-2", model.DomainRoleAdmin); err != nil {
		t.Fatalf("SetMemberRole: %v", err)
	}
	member, err := repository.NewDomainRepository(db).GetMember(domain.UUID, "u-2")
	if err != nil {
		t.Fatalf("GetMember: %v", err)
	}
	if member.RoleName != model.DomainRoleAdmin {
		t.Fatalf("expected admin, got %s", member.RoleName)
	}
	if err := svc.SetMemberRole(domain.UUID, "u-2", "u-2", model.DomainRoleGuest); err == nil {
		t.Fatal("must reject changing own role")
	}
	if err := svc.SetMemberRole(domain.UUID, "owner-1", "u-2", model.DomainRoleOwner); err == nil {
		t.Fatal("must reject assigning owner role")
	}
}
