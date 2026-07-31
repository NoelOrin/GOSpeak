package service

import (
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

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
