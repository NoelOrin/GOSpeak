package repository

import (
	"testing"

	"GOSpeak/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newUserGroupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.UserGroup{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestUserGroupRepo_ListAndRename(t *testing.T) {
	db := newUserGroupTestDB(t)
	repo := NewUserGroupRepository(db)

	groups := []*model.UserGroup{
		{UserID: 1, GroupName: "朋友"},
		{UserID: 1, GroupName: "队友"},
		{UserID: 2, GroupName: "朋友"},
	}
	for _, group := range groups {
		if err := repo.Create(group); err != nil {
			t.Fatal(err)
		}
	}

	got, err := repo.ListByUser(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 groups, got %d", len(got))
	}
	if got[0].GroupName != "朋友" || got[1].GroupName != "队友" {
		t.Fatalf("unexpected order: %+v", got)
	}

	if err := repo.Rename(got[0].ID, "密友"); err != nil {
		t.Fatal(err)
	}
	updated, err := repo.GetByID(got[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.GroupName != "密友" {
		t.Fatalf("want renamed group, got %q", updated.GroupName)
	}

	if err := repo.Delete(updated.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByID(updated.ID); err != gorm.ErrRecordNotFound {
		t.Fatalf("want record not found, got %v", err)
	}
}
