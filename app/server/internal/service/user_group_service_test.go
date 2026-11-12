package service

import (
	"strings"
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newUserGroupServiceTest(t *testing.T) *UserGroupService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.UserGroup{}); err != nil {
		t.Fatal(err)
	}
	return NewUserGroupService(repository.NewUserGroupRepository(db))
}

func TestUserGroupService_CreateAndList(t *testing.T) {
	svc := newUserGroupServiceTest(t)

	group, err := svc.Create(1, "  朋友  ")
	if err != nil {
		t.Fatal(err)
	}
	if group.GroupName != "朋友" {
		t.Fatalf("want trimmed name, got %q", group.GroupName)
	}
	groups, err := svc.List(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].GroupName != "朋友" {
		t.Fatalf("unexpected groups: %+v", groups)
	}
}

func TestUserGroupService_CreateDuplicate(t *testing.T) {
	svc := newUserGroupServiceTest(t)
	if _, err := svc.Create(1, "朋友"); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Create(1, "朋友")
	assertErrorCode(t, err, pkg.ALREADY_EXISTS)

	if _, err := svc.Create(2, "朋友"); err != nil {
		t.Fatalf("different user should be allowed: %v", err)
	}
}

func TestUserGroupService_CreateValidation(t *testing.T) {
	svc := newUserGroupServiceTest(t)
	if _, err := svc.Create(1, "  "); err == nil {
		t.Fatal("want error for empty group name")
	}
	long := strings.Repeat("组", MaxUserGroupNameRunes+1)
	if _, err := svc.Create(1, long); err == nil {
		t.Fatal("want error for long group name")
	}
}

func TestUserGroupService_RenameOwnedGroup(t *testing.T) {
	svc := newUserGroupServiceTest(t)
	group, err := svc.Create(1, "朋友")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Rename(group.ID, 1, "队友"); err != nil {
		t.Fatal(err)
	}
	groups, err := svc.List(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].GroupName != "队友" {
		t.Fatalf("unexpected groups: %+v", groups)
	}
}

func TestUserGroupService_RenameRejectsOtherOwner(t *testing.T) {
	svc := newUserGroupServiceTest(t)
	group, err := svc.Create(1, "朋友")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Rename(group.ID, 2, "队友"); err == nil {
		t.Fatal("want ownership error")
	}
}

func TestUserGroupService_DeleteOwnedGroup(t *testing.T) {
	svc := newUserGroupServiceTest(t)
	group, err := svc.Create(1, "朋友")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(group.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.List(1); err != nil {
		t.Fatal(err)
	}
	groups, _ := svc.List(1)
	if len(groups) != 0 {
		t.Fatalf("want empty list, got %+v", groups)
	}
}
