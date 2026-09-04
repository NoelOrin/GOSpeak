package service

import (
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRoomService_CreateRoom_RejectsDuplicateNameInSameDomain(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	svc := NewRoomService(repository.NewRoomRepository(db))
	if _, err := svc.CreateRoom("lobby", "", "", 10, true, true, "user-1", "voice", "domain-a"); err != nil {
		t.Fatalf("first CreateRoom: %v", err)
	}

	_, err = svc.CreateRoom("lobby", "", "", 10, true, true, "user-2", "voice", "domain-a")
	if err == nil {
		t.Fatal("expected duplicate room in same domain to fail")
	}
	appErr, ok := err.(*pkg.AppError)
	if !ok || appErr.Code != pkg.ALREADY_EXISTS {
		t.Fatalf("expected ALREADY_EXISTS, got %#v", err)
	}

	if _, err := svc.CreateRoom("lobby", "", "", 10, true, true, "user-3", "voice", "domain-b"); err != nil {
		t.Fatalf("same name in different domain should succeed: %v", err)
	}
}

func TestRoomService_CreateRoom_RejectsEmptyDomainUUID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	svc := NewRoomService(repository.NewRoomRepository(db))
	_, err = svc.CreateRoom("general", "", "", 10, true, true, "user-1", "voice", "")
	if err == nil {
		t.Fatal("expected empty domain_uuid to fail")
	}
	appErr, ok := err.(*pkg.AppError)
	if !ok || appErr.Code != pkg.INVALID_PARAMS {
		t.Fatalf("expected INVALID_PARAMS, got %#v", err)
	}
}
