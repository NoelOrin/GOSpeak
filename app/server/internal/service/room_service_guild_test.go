package service

import (
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRoomService_List_FiltersGuildUUID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	rooms := []model.Room{
		{Name: "lobby", GuildUUID: "guild-a"},
		{Name: "lobby", GuildUUID: "guild-b"},
		{Name: "general"},
	}
	if err := db.Create(&rooms).Error; err != nil {
		t.Fatalf("seed rooms: %v", err)
	}

	svc := NewRoomService(repository.NewRoomRepository(db))
	got, total, err := svc.List(1, 100, "", "guild-a")
	if err != nil {
		t.Fatalf("List guild-a: %v", err)
	}
	if total != 1 || len(got) != 1 || got[0].GuildUUID != "guild-a" {
		t.Fatalf("expected 1 guild-a room, got total=%d rooms=%+v", total, got)
	}

	all, allTotal, err := svc.List(1, 100, "", "")
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if allTotal != 3 || len(all) != 3 {
		t.Fatalf("expected all rooms, got total=%d rooms=%d", allTotal, len(all))
	}
}

func TestRoomService_CreateRoom_PersistsGuildUUID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	svc := NewRoomService(repository.NewRoomRepository(db))
	room, err := svc.CreateRoom("lobby", "", "desc", 10, true, true, "user-1", "voice", "guild-a")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	var saved model.Room
	if err := db.Where("uuid = ?", room.UUID).First(&saved).Error; err != nil {
		t.Fatalf("load room: %v", err)
	}
	if saved.GuildUUID != "guild-a" {
		t.Fatalf("expected guild_uuid guild-a, got %q", saved.GuildUUID)
	}
}

func TestRoomService_ListPlatform_OnlyPlatformRooms(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	rooms := []model.Room{
		{Name: "guild-lobby", GuildUUID: "guild-a"},
		{Name: "general"},
	}
	if err := db.Create(&rooms).Error; err != nil {
		t.Fatalf("seed rooms: %v", err)
	}

	svc := NewRoomService(repository.NewRoomRepository(db))
	got, total, err := svc.ListPlatform(1, 100, "")
	if err != nil {
		t.Fatalf("ListPlatform: %v", err)
	}
	if total != 1 || len(got) != 1 || got[0].GuildUUID != "" || got[0].Name != "general" {
		t.Fatalf("expected only platform room, got total=%d rooms=%+v", total, got)
	}
}
