package service

import (
	"testing"

	"GOSpeak/internal/config"
	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSFUConfigTestRepo(t *testing.T) *repository.SFUConfigRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.SFUConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return repository.NewSFUConfigRepository(db)
}

func TestSyncFromEnv_OverwritesExistingDBRow(t *testing.T) {
	repo := newSFUConfigTestRepo(t)
	if err := repo.Save(&model.SFUConfig{
		ID:          1,
		Provider:    "livekit",
		LiveKitHost: "stale-host",
	}); err != nil {
		t.Fatalf("seed stale row: %v", err)
	}

	baseCfg := &config.Config{
		SFUProvider: "srs",
		SRSHost:     "localhost",
		SRSApiPort:  "1985",
		SRSWHIPPort: "1985",
	}
	svc := NewSFUConfigService(repo, baseCfg)

	if err := svc.SyncFromEnv(); err != nil {
		t.Fatalf("sync from env: %v", err)
	}

	got, err := repo.Get()
	if err != nil {
		t.Fatalf("get after sync: %v", err)
	}
	if got.Provider != "srs" {
		t.Errorf("provider = %q, want srs", got.Provider)
	}
	if got.SRSHost != "localhost" {
		t.Errorf("srs_host = %q, want localhost", got.SRSHost)
	}
	if got.LiveKitHost != "" {
		t.Errorf("livekit_host = %q, want empty (env overwrites stale DB)", got.LiveKitHost)
	}
}

func TestSyncFromEnv_SeedsWhenDBEmpty(t *testing.T) {
	repo := newSFUConfigTestRepo(t)
	baseCfg := &config.Config{
		SFUProvider: "srs",
		SRSHost:     "localhost",
		SRSApiPort:  "1985",
		SRSWHIPPort: "1985",
	}
	svc := NewSFUConfigService(repo, baseCfg)

	if err := svc.SyncFromEnv(); err != nil {
		t.Fatalf("sync from env: %v", err)
	}

	got, err := repo.Get()
	if err != nil {
		t.Fatalf("get after sync: %v", err)
	}
	if got.Provider != "srs" {
		t.Errorf("provider = %q, want srs", got.Provider)
	}
}
