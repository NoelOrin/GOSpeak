package repository

import (
	"testing"

	"GOSpeak/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newPermissionTestRepo(t *testing.T) *PermissionRepository {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Permission{}, &model.RolePermission{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := NewPermissionRepository(db)
	for i := range model.DefaultPermissions {
		perm := model.DefaultPermissions[i]
		if err := repo.CreateIfNotExists(&perm); err != nil {
			t.Fatalf("seed permission %s: %v", perm.Code, err)
		}
	}
	return repo
}

func TestSeedRolePermissionsIfEmptyKeepsExistingRolePermissions(t *testing.T) {
	repo := newPermissionTestRepo(t)

	if err := repo.SyncRolePermissions("user", []string{model.PermRoomRead}); err != nil {
		t.Fatalf("sync role permissions: %v", err)
	}
	if err := repo.SeedRolePermissionsIfEmpty("user", model.DefaultRolePermissions["user"]); err != nil {
		t.Fatalf("seed role permissions: %v", err)
	}

	codes, err := repo.GetRolePermissions("user")
	if err != nil {
		t.Fatalf("get role permissions: %v", err)
	}
	if len(codes) != 1 || codes[0] != model.PermRoomRead {
		t.Fatalf("expected existing permissions to remain unchanged, got %#v", codes)
	}
}

func TestSeedRolePermissionsIfEmptySeedsMissingRolePermissions(t *testing.T) {
	repo := newPermissionTestRepo(t)

	if err := repo.SeedRolePermissionsIfEmpty("admin", model.DefaultRolePermissions["admin"]); err != nil {
		t.Fatalf("seed role permissions: %v", err)
	}

	codes, err := repo.GetRolePermissions("admin")
	if err != nil {
		t.Fatalf("get role permissions: %v", err)
	}
	if len(codes) != len(model.DefaultRolePermissions["admin"]) {
		t.Fatalf("expected %d permissions, got %d", len(model.DefaultRolePermissions["admin"]), len(codes))
	}
}
