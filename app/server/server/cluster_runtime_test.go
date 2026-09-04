package server

import (
	"testing"

	"GOSpeak/internal/config"
	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/service"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestStartLocalClusterRuntime_DeregistersOnStopOnly(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ClusterNode{}, &model.ServerAssignment{}); err != nil {
		t.Fatal(err)
	}
	clusterSvc := service.NewClusterService(
		repository.NewClusterNodeRepository(db),
		repository.NewServerAssignmentRepository(db),
	)

	cfg := &config.Config{
		ClusterRole:              model.ClusterRoleAll,
		ClusterNodeID:            "local-node-1",
		ClusterHeartbeatInterval: "1h",
		ClusterHeartbeatTimeout:  "1h",
		ClusterMaxServers:        10,
		ClusterMaxRooms:          100,
	}

	nodeID, stop, err := startLocalClusterRuntime(cfg, clusterSvc, nil, "inst-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if nodeID != "local-node-1" {
		t.Fatalf("node id = %q", nodeID)
	}

	nodeRepo := repository.NewClusterNodeRepository(db)
	node, err := nodeRepo.GetByUUID(nodeID)
	if err != nil {
		t.Fatal(err)
	}
	if node.Status == model.ClusterNodeOffline {
		t.Fatal("node must not be offline right after startup")
	}

	stop()

	node, err = nodeRepo.GetByUUID(nodeID)
	if err != nil {
		t.Fatal(err)
	}
	if node.Status != model.ClusterNodeOffline {
		t.Fatalf("node status after stop = %q, want offline", node.Status)
	}
}

func newClusterRuntimeDB(t *testing.T) (*gorm.DB, *service.ClusterService) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ClusterNode{}, &model.ServerAssignment{}); err != nil {
		t.Fatal(err)
	}
	return db, service.NewClusterService(
		repository.NewClusterNodeRepository(db),
		repository.NewServerAssignmentRepository(db),
	)
}

func TestStartClusterRuntimes_EmbeddedBusSkipsLeaderFence(t *testing.T) {
	db, clusterSvc := newClusterRuntimeDB(t)
	cfg := &config.Config{
		ClusterRole:              model.ClusterRoleAll,
		ClusterNodeID:            "local-node-embedded",
		ClusterHeartbeatInterval: "1h",
		ClusterHeartbeatTimeout:  "1h",
		ClusterMaxServers:        10,
		ClusterMaxRooms:          100,
	}
	_, stop, _, _, leaderFence, _, degradedToWorker, err := startClusterRuntimes(cfg, db, nil, "inst-embedded", clusterSvc, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if leaderFence != nil {
		t.Fatal("embedded bus must not create a leader fence")
	}
	if degradedToWorker {
		t.Fatal("embedded bus must not degrade to worker")
	}
}

func TestStartClusterRuntimes_ExternalBusWithoutConnDegradesToWorker(t *testing.T) {
	db, clusterSvc := newClusterRuntimeDB(t)
	cfg := &config.Config{
		ClusterRole:              model.ClusterRoleAll,
		ClusterNodeID:            "local-node-external",
		NATSURL:                  "nats://127.0.0.1:4222",
		ClusterHeartbeatInterval: "1h",
		ClusterHeartbeatTimeout:  "1h",
		ClusterMaxServers:        10,
		ClusterMaxRooms:          100,
	}
	_, stop, _, _, leaderFence, _, degradedToWorker, err := startClusterRuntimes(cfg, db, nil, "inst-external", clusterSvc, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if leaderFence != nil {
		t.Fatal("lock-unavailable instance must not hold a leader fence")
	}
	if !degradedToWorker {
		t.Fatal("lock-unavailable instance must degrade to worker")
	}
}
