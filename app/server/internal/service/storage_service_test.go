package service

import (
	"testing"

	"GOSpeak/internal/config"
	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestReloadProvider_RebuildsProvider(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.StorageConfig{}); err != nil {
		t.Fatalf("migrate storage config: %v", err)
	}
	repo := repository.NewStorageConfigRepository(db)
	svc := NewStorageService(repo, &config.Config{StorageType: "local", StoragePathPrefix: "uploads/"})

	first, err := svc.ReloadProvider()
	if err != nil {
		t.Fatalf("first reload: %v", err)
	}
	if first == nil {
		t.Fatal("expected first provider")
	}

	second, err := svc.ReloadProvider()
	if err != nil {
		t.Fatalf("second reload: %v", err)
	}
	if second == first {
		t.Fatal("ReloadProvider must rebuild the provider instead of returning the cached instance")
	}
}
