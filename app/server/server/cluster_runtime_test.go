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
