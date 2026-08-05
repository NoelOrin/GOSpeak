package service

import (
	"sync"
	"testing"
	"time"

	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newMuteServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Mute{}); err != nil {
		t.Fatalf("migrate mute: %v", err)
	}
	return db
}

func TestMuteService_ExpiredTemporaryMuteRunsFullUnmute(t *testing.T) {
	db := newMuteServiceTestDB(t)
	muteRepo := repository.NewMuteRepository(db)
	userRepo := repository.NewUserRepository(db)
	svc := NewMuteService(muteRepo, userRepo)

	expired := time.Now().Add(-time.Minute)
	if err := muteRepo.Create(&model.Mute{
		UserID:    42,
		Duration:  60,
		Permanent: false,
		ExpiresAt: &expired,
		Reason:    "temp mute",
	}); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	calls := 0
	svc.SetOnExpired(func(userID uint) {
		mu.Lock()
		calls++
		mu.Unlock()
		if userID != 42 {
			t.Fatalf("expected expired user 42, got %d", userID)
		}
	})

	muted, _, err := svc.IsMuted(42)
	if err != nil {
		t.Fatal(err)
	}
	if muted {
		t.Fatal("expired mute should no longer be active")
	}

	mu.Lock()
	got := calls
	mu.Unlock()
	if got != 1 {
		t.Fatalf("expected 1 unmute callback, got %d", got)
	}

	if _, err := muteRepo.GetByUserID(42); err == nil {
		t.Fatal("expired mute record should be deleted")
	}

	if _, _, err := svc.IsMuted(42); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	got = calls
	mu.Unlock()
	if got != 1 {
		t.Fatalf("expected no duplicate callback, got %d", got)
	}
}
