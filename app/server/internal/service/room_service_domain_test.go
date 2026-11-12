package service

import (
	"testing"

	"fmt"

	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRoomService_List_FiltersDomainUUID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	rooms := []model.Room{
		{Name: "lobby", DomainUUID: "domain-a"},
		{Name: "lobby", DomainUUID: "domain-b"},
		{Name: "general"},
	}
	if err := db.Create(&rooms).Error; err != nil {
		t.Fatalf("seed rooms: %v", err)
	}

	svc := NewRoomService(repository.NewRoomRepository(db))
	got, total, err := svc.List(1, 100, "", "domain-a")
	if err != nil {
		t.Fatalf("List domain-a: %v", err)
	}
	if total != 1 || len(got) != 1 || got[0].DomainUUID != "domain-a" {
		t.Fatalf("expected 1 domain-a room, got total=%d rooms=%+v", total, got)
	}

	all, allTotal, err := svc.List(1, 100, "", "")
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if allTotal != 3 || len(all) != 3 {
		t.Fatalf("expected all rooms, got total=%d rooms=%d", allTotal, len(all))
	}
}

func TestRoomService_List_PageSizeCapsAt100(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for i := 0; i < 120; i++ {
		room := model.Room{Name: fmt.Sprintf("room-%03d", i), DomainUUID: "domain-a"}
		if err := db.Create(&room).Error; err != nil {
			t.Fatalf("seed room %d: %v", i, err)
		}
	}

	svc := NewRoomService(repository.NewRoomRepository(db))
	got, total, err := svc.List(1, 200, "", "domain-a")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 120 || len(got) != 100 {
		t.Fatalf("expected total=120 rooms=100, got total=%d rooms=%d", total, len(got))
	}
}

func TestRoomService_CreateRoom_PersistsDomainUUID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	svc := NewRoomService(repository.NewRoomRepository(db))
	room, err := svc.CreateRoom("lobby", "", "desc", 10, true, true, "user-1", "voice", "domain-a")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	var saved model.Room
	if err := db.Where("uuid = ?", room.UUID).First(&saved).Error; err != nil {
		t.Fatalf("load room: %v", err)
	}
	if saved.DomainUUID != "domain-a" {
		t.Fatalf("expected domain_uuid domain-a, got %q", saved.DomainUUID)
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
		{Name: "domain-lobby", DomainUUID: "domain-a"},
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
	if total != 1 || len(got) != 1 || got[0].DomainUUID != "" || got[0].Name != "general" {
		t.Fatalf("expected only platform room, got total=%d rooms=%+v", total, got)
	}
}
