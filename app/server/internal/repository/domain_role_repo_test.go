package repository

import (
	"testing"

	"GOSpeak/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newDomainRoleTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Domain{},
		&model.DomainMember{},
		&model.DomainRole{},
		&model.DomainRolePermission{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestSeedDefaultDomainRoles_CreatesSystemRoles(t *testing.T) {
	db := newDomainRoleTestDB(t)
	domain := &model.Domain{Name: "Test", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("SeedDefaultDomainRoles: %v", err)
	}

	repo := NewDomainRoleRepository(db)
	roles, err := repo.ListRoles(domain.UUID)
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	if len(roles) != 4 {
		t.Fatalf("expected 4 system roles, got %d", len(roles))
	}
	for _, name := range []string{model.DomainRoleAdmin, model.DomainRoleMember, model.DomainRoleGuest} {
		codes, err := repo.GetRolePermissions(domain.UUID, name)
		if err != nil {
			t.Fatalf("GetRolePermissions(%s): %v", name, err)
		}
		if len(codes) == 0 {
			t.Errorf("role %s must have seeded permissions", name)
		}
	}
	ownerCodes, err := repo.GetRolePermissions(domain.UUID, model.DomainRoleOwner)
	if err != nil {
		t.Fatalf("owner permissions: %v", err)
	}
	if len(ownerCodes) != 0 {
		t.Errorf("owner must not have stored permission rows, got %v", ownerCodes)
	}
}

func TestDomainRoleRepository_CreateAndDelete(t *testing.T) {
	db := newDomainRoleTestDB(t)
	repo := NewDomainRoleRepository(db)

	role := &model.DomainRole{DomainUUID: "domain-a", Name: "moderator"}
	if err := repo.CreateRoleWithPermissions(role, []string{model.PermRoomRead, model.PermMessageRead}); err != nil {
		t.Fatalf("CreateRoleWithPermissions: %v", err)
	}
	codes, err := repo.GetRolePermissions("domain-a", "moderator")
	if err != nil {
		t.Fatalf("GetRolePermissions: %v", err)
	}
	if len(codes) != 2 {
		t.Fatalf("expected 2 permissions, got %v", codes)
	}

	inUse, err := repo.RoleInUse("domain-a", "moderator")
	if err != nil {
		t.Fatalf("RoleInUse: %v", err)
	}
	if inUse {
		t.Fatal("moderator must not be in use")
	}
	if err := db.Create(&model.DomainMember{
		DomainUUID: "domain-a", UserUUID: "u-1", RoleName: "moderator",
	}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	inUse, err = repo.RoleInUse("domain-a", "moderator")
	if err != nil {
		t.Fatalf("RoleInUse after member: %v", err)
	}
	if !inUse {
		t.Fatal("moderator must be in use after member assignment")
	}

	if err := repo.DeleteRole("domain-a", "moderator"); err != nil {
		t.Fatalf("DeleteRole: %v", err)
	}
	if _, err := repo.GetRole("domain-a", "moderator"); err == nil {
		t.Fatal("expected role to be deleted")
	}
}

func TestDomainRoleRepository_SyncPermissions(t *testing.T) {
	db := newDomainRoleTestDB(t)
	repo := NewDomainRoleRepository(db)
	role := &model.DomainRole{DomainUUID: "domain-b", Name: "viewer"}
	if err := repo.CreateRoleWithPermissions(role, []string{model.PermRoomRead}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.SyncRolePermissions("domain-b", "viewer", []string{model.PermMessageRead, model.PermRoomCreate}); err != nil {
		t.Fatalf("sync: %v", err)
	}
	codes, err := repo.GetRolePermissions("domain-b", "viewer")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(codes) != 2 {
		t.Fatalf("expected 2 codes after sync, got %v", codes)
	}
}

func TestBackfillDomainRoleDefaults(t *testing.T) {
	db := newDomainRoleTestDB(t)
	repo := NewDomainRoleRepository(db)

	legacy := &model.Domain{Name: "Legacy", OwnerUUID: "owner-1"}
	seeded := &model.Domain{Name: "Seeded", OwnerUUID: "owner-2"}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatalf("seed legacy domain: %v", err)
	}
	if err := db.Create(seeded).Error; err != nil {
		t.Fatalf("seed seeded domain: %v", err)
	}
	if err := SeedDefaultDomainRoles(db, seeded.UUID); err != nil {
		t.Fatalf("pre-seed: %v", err)
	}

	if err := BackfillDomainRoleDefaults(db); err != nil {
		t.Fatalf("BackfillDomainRoleDefaults: %v", err)
	}
	if err := BackfillDomainRoleDefaults(db); err != nil {
		t.Fatalf("BackfillDomainRoleDefaults repeat: %v", err)
	}

	var total int64
	if err := db.Model(&model.DomainRole{}).Count(&total).Error; err != nil {
		t.Fatalf("count roles: %v", err)
	}
	if total != 8 {
		t.Fatalf("expected 8 roles across 2 domains, got %d", total)
	}
	for _, domainUUID := range []string{legacy.UUID, seeded.UUID} {
		roles, err := repo.ListRoles(domainUUID)
		if err != nil {
			t.Fatalf("ListRoles(%s): %v", domainUUID, err)
		}
		if len(roles) != 4 {
			t.Fatalf("domain %s expected 4 roles, got %d", domainUUID, len(roles))
		}
	}
}
