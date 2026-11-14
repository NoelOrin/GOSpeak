package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"errors"
	"strings"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
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

func TestMuteService_TemporaryMuteRequiresDuration(t *testing.T) {
	db := newMuteServiceTestDB(t)
	muteRepo := repository.NewMuteRepository(db)
	userRepo := repository.NewUserRepository(db)
	svc := NewMuteService(muteRepo, userRepo)

	for _, duration := range []int64{0, -1} {
		_, err := svc.MuteUser(1, 42, duration, false, "temp mute")
		var appErr *pkg.AppError
		if !errors.As(err, &appErr) || appErr.Code != pkg.INVALID_PARAMS {
			t.Fatalf("duration=%d: expected INVALID_PARAMS, got %v", duration, err)
		}
	}
}

func TestMuteService_RejectsLongReason(t *testing.T) {
	db := newMuteServiceTestDB(t)
	muteRepo := repository.NewMuteRepository(db)
	userRepo := repository.NewUserRepository(db)
	svc := NewMuteService(muteRepo, userRepo)

	_, err := svc.MuteUser(1, 42, 0, true, strings.Repeat("x", 501))
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) || appErr.Code != pkg.INVALID_PARAMS {
		t.Fatalf("expected INVALID_PARAMS, got %v", err)
	}
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
	var events []string
	svc.SetEventBus(eventPublishRecorder{
		fn: func(event string, payload interface{}) error {
			if event == EventMuteExpired {
				m, _ := payload.(MuteExpiredPayload)
				if m.UserID != "42" {
					t.Fatalf("expected user_id 42, got %+v", m)
				}
			}
			mu.Lock()
			events = append(events, event)
			mu.Unlock()
			return nil
		},
	})

	muted, _, err := svc.IsMuted(42)
	if err != nil {
		t.Fatal(err)
	}
	if muted {
		t.Fatal("expired mute should no longer be active")
	}

	mu.Lock()
	got := events[len(events)-1]
	mu.Unlock()
	if got != EventMuteExpired {
		t.Fatalf("expected final %s event, got %q", EventMuteExpired, got)
	}

	if _, err := muteRepo.GetByUserID(42); err == nil {
		t.Fatal("expired mute record should be deleted")
	}

	if _, _, err := svc.IsMuted(42); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	allEvents := events
	mu.Unlock()
	expiredCount := 0
	for _, e := range allEvents {
		if e == EventMuteExpired {
			expiredCount++
		}
	}
	if expiredCount != 1 {
		t.Fatalf("expected exactly one expiry event, got %d in %v", expiredCount, allEvents)
	}
}

func TestMuteService_CacheHitAvoidsUserLookup(t *testing.T) {
	db := newMuteServiceTestDB(t)
	muteRepo := repository.NewMuteRepository(db)
	userRepo := repository.NewUserRepository(db)
	svc := NewMuteService(muteRepo, userRepo)

	permanent := true
	svc.cacheMute(42, "alice", true, &model.Mute{UserID: 42, Permanent: permanent})
	muted, mute, err := svc.IsMutedByIdentity("alice")
	if err != nil {
		t.Fatalf("IsMutedByIdentity: %v", err)
	}
	if !muted || mute == nil || !mute.Permanent {
		t.Fatalf("expected cached permanent mute, got muted=%v mute=%+v", muted, mute)
	}

	svc.invalidateMute(42, "alice", false)
	if _, ok := svc.byIdentity["alice"]; ok {
		t.Fatal("expected identity cache entry to be removed")
	}
	if _, ok := svc.byUserID[42]; ok {
		t.Fatal("expected user id cache entry to be removed")
	}
}

type eventPublishRecorder struct {
	fn func(event string, payload interface{}) error
}

func (r eventPublishRecorder) PublishInternal(_ context.Context, event string, payload interface{}) error {
	return r.fn(event, payload)
}
