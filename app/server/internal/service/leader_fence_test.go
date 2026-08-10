package service

import (
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newLeaderFenceService(t *testing.T, leaderID string) *LeaderFenceService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ClusterLeaderFence{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewLeaderFenceService(repository.NewClusterFenceRepository(db), leaderID)
}

func TestLeaderFence_VerifyFailsAfterTakeover(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ClusterLeaderFence{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := repository.NewClusterFenceRepository(db)
	svcA := NewLeaderFenceService(repo, "agent-a")
	if err := svcA.Acquire(); err != nil {
		t.Fatalf("acquire A: %v", err)
	}
	if err := svcA.Verify(); err != nil {
		t.Fatalf("verify A: %v", err)
	}

	svcB := NewLeaderFenceService(repo, "agent-b")
	if err := svcB.Acquire(); err != nil {
		t.Fatalf("acquire B: %v", err)
	}

	if err := svcA.Verify(); err == nil {
		t.Fatal("old leader verify must fail after takeover")
	}
	if svcA.Active() {
		t.Fatal("old leader should be deactivated after failed verify")
	}
}

func TestLeaderFence_DeactivateBlocksVerify(t *testing.T) {
	svc := newLeaderFenceService(t, "agent-a")
	if err := svc.Acquire(); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	svc.Deactivate()
	if err := svc.Verify(); err == nil {
		t.Fatal("verify must fail after deactivate")
	}
}
