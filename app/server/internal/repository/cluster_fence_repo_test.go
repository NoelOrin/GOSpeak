package repository

import (
	"testing"

	"GOSpeak/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newFenceRepo(t *testing.T) *ClusterFenceRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ClusterLeaderFence{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewClusterFenceRepository(db)
}

func TestClusterFence_AcquireInvalidatesOldLeader(t *testing.T) {
	repo := newFenceRepo(t)

	epochA, err := repo.Acquire("agent-a")
	if err != nil {
		t.Fatalf("acquire agent-a: %v", err)
	}
	if epochA != 1 {
		t.Fatalf("epochA = %d, want 1", epochA)
	}
	ok, err := repo.Verify("agent-a", epochA)
	if err != nil || !ok {
		t.Fatalf("verify agent-a: ok=%v err=%v", ok, err)
	}

	epochB, err := repo.Acquire("agent-b")
	if err != nil {
		t.Fatalf("acquire agent-b: %v", err)
	}
	if epochB != 2 {
		t.Fatalf("epochB = %d, want 2", epochB)
	}
	ok, err = repo.Verify("agent-a", epochA)
	if err != nil {
		t.Fatalf("old leader verify error: %v", err)
	}
	if ok {
		t.Fatal("old leader must not pass DB fence after takeover")
	}
	ok, err = repo.Verify("agent-b", epochB)
	if err != nil || !ok {
		t.Fatalf("verify agent-b: ok=%v err=%v", ok, err)
	}
}
