package repository

import (
	"testing"

	"GOSpeak/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openRoomTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}); err != nil {
		t.Fatalf("migrate room: %v", err)
	}
	return db
}

func TestRoomRepository_List_FiltersGuildUUID(t *testing.T) {
	db := openRoomTestDB(t)
	rooms := []model.Room{
		{Name: "lobby", GuildUUID: "guild-a"},
		{Name: "lobby", GuildUUID: "guild-b"},
		{Name: "general"},
	}
	if err := db.Create(&rooms).Error; err != nil {
		t.Fatalf("seed rooms: %v", err)
	}

	repo := NewRoomRepository(db)
	got, total, err := repo.List(1, 100, "", "guild-a")
	if err != nil {
		t.Fatalf("List guild-a: %v", err)
	}
	if total != 1 || len(got) != 1 {
		t.Fatalf("expected 1 guild-a room, got total=%d rooms=%d", total, len(got))
	}
	if got[0].GuildUUID != "guild-a" || got[0].Name != "lobby" {
		t.Fatalf("unexpected room: %+v", got[0])
	}

	all, allTotal, err := repo.List(1, 100, "", "")
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if allTotal != 3 || len(all) != 3 {
		t.Fatalf("expected all 3 rooms, got total=%d rooms=%d", allTotal, len(all))
	}
}

func TestRoomRepository_ListPlatform_OnlyPlatformRooms(t *testing.T) {
	db := openRoomTestDB(t)
	rooms := []model.Room{
		{Name: "guild-lobby", GuildUUID: "guild-a"},
		{Name: "general"},
	}
	if err := db.Create(&rooms).Error; err != nil {
		t.Fatalf("seed rooms: %v", err)
	}

	repo := NewRoomRepository(db)
	got, total, err := repo.ListPlatform(1, 100, "")
	if err != nil {
		t.Fatalf("ListPlatform: %v", err)
	}
	if total != 1 || len(got) != 1 || got[0].GuildUUID != "" || got[0].Name != "general" {
		t.Fatalf("expected only platform room, got total=%d rooms=%+v", total, got)
	}
}
