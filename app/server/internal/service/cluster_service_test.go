package service

import (
	"testing"
	"time"

	"context"

	"GOSpeak/internal/cluster"
	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupClusterServiceTestDB(t *testing.T) (*ClusterService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ClusterNode{}, &model.ServerAssignment{}); err != nil {
		t.Fatalf("migrate cluster models: %v", err)
	}
	svc := NewClusterService(
		repository.NewClusterNodeRepository(db),
		repository.NewServerAssignmentRepository(db),
	)
	if err := db.AutoMigrate(&model.Domain{}); err != nil {
		t.Fatalf("migrate domain model: %v", err)
	}
	svc.SetServerRepo(repository.NewDomainRepository(db))
	return svc, db
}

func TestClusterService_RegisterAndHeartbeat(t *testing.T) {
	svc, _ := setupClusterServiceTestDB(t)

	node, err := svc.RegisterNode(model.ClusterNode{
		UUID:        "node-a",
		Name:        "worker-a",
		Host:        "worker-a.example",
		Role:        model.ClusterRoleWorker,
		SFUProvider: "livekit",
		MaxServers:  10,
		MaxRooms:    100,
	})
	if err != nil {
		t.Fatalf("RegisterNode: %v", err)
	}
	if node.Status != model.ClusterNodePending {
		t.Fatalf("expected pending after register, got %q", node.Status)
	}

	healthy := true
	updated, err := svc.Heartbeat(node.UUID, cluster.HeartbeatReport{
		NodeID:      node.UUID,
		Status:      model.ClusterNodeReady,
		Rooms:       5,
		Connections: 12,
		LoadPercent: 20,
		SFUHealthy:  &healthy,
	})
	if err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}
	if updated.Status != model.ClusterNodeReady {
		t.Fatalf("expected ready after heartbeat, got %q", updated.Status)
	}
	if updated.Rooms != 5 || updated.Connections != 12 {
		t.Fatalf("heartbeat stats not saved: %+v", updated)
	}
	if updated.LastSeenAt.IsZero() {
		t.Fatal("expected LastSeenAt to be refreshed")
	}
}

func TestClusterService_ScaleServer(t *testing.T) {
	svc, db := setupClusterServiceTestDB(t)
	nodeRepo := repository.NewClusterNodeRepository(db)

	nodes := []model.ClusterNode{
		{UUID: "node-a", Name: "a", Status: model.ClusterNodeReady, SFUHealthy: true, MaxServers: 10, MaxRooms: 100, LoadPercent: 10},
		{UUID: "node-b", Name: "b", Status: model.ClusterNodeReady, SFUHealthy: true, MaxServers: 10, MaxRooms: 100, LoadPercent: 50},
	}
	for i := range nodes {
		if err := nodeRepo.Create(&nodes[i]); err != nil {
			t.Fatalf("create node %d: %v", i, err)
		}
	}

	assignments, err := svc.ScaleServer("srv-1", 2, "node-a")
	if err != nil {
		t.Fatalf("ScaleServer up: %v", err)
	}
	if len(assignments) != 2 {
		t.Fatalf("expected 2 assignments, got %d", len(assignments))
	}

	assignments, err = svc.ScaleServer("srv-1", 1, "node-a")
	if err != nil {
		t.Fatalf("ScaleServer down: %v", err)
	}
	if len(assignments) != 1 || assignments[0].NodeUUID != "node-a" {
		t.Fatalf("expected node-a to remain, got %+v", assignments)
	}

	if err := svc.DeleteServer("srv-1"); err != nil {
		t.Fatalf("DeleteServer: %v", err)
	}
	assignments, err = svc.ListAssignments("srv-1")
	if err != nil {
		t.Fatalf("ListAssignments: %v", err)
	}
	if len(assignments) != 0 {
		t.Fatalf("expected assignments deleted, got %d", len(assignments))
	}
}

func TestClusterService_ScaleServerReplacesOfflineAssignment(t *testing.T) {
	svc, db := setupClusterServiceTestDB(t)
	nodeRepo := repository.NewClusterNodeRepository(db)
	assignRepo := repository.NewServerAssignmentRepository(db)

	nodes := []model.ClusterNode{
		{UUID: "node-old", Name: "old", Status: model.ClusterNodeOffline, SFUHealthy: false, MaxServers: 10, MaxRooms: 100},
		{UUID: "node-new", Name: "new", Status: model.ClusterNodeReady, SFUHealthy: true, MaxServers: 10, MaxRooms: 100},
	}
	for i := range nodes {
		if err := nodeRepo.Create(&nodes[i]); err != nil {
			t.Fatalf("create node %d: %v", i, err)
		}
	}
	if err := assignRepo.Ensure("srv-restart", "node-old"); err != nil {
		t.Fatalf("seed stale assignment: %v", err)
	}

	assignments, err := svc.ScaleServer("srv-restart", 1, "node-new")
	if err != nil {
		t.Fatalf("ScaleServer: %v", err)
	}
	if len(assignments) != 1 || assignments[0].NodeUUID != "node-new" {
		t.Fatalf("expected stale assignment replaced by node-new, got %+v", assignments)
	}
}

func TestClusterService_DrainAndUndrain(t *testing.T) {
	svc, db := setupClusterServiceTestDB(t)
	nodeRepo := repository.NewClusterNodeRepository(db)
	node := &model.ClusterNode{UUID: "node-drain", Name: "drain", Status: model.ClusterNodeReady, SFUHealthy: true, MaxServers: 10, MaxRooms: 100}
	if err := nodeRepo.Create(node); err != nil {
		t.Fatalf("create node: %v", err)
	}
	if err := svc.DrainNode(node.UUID); err != nil {
		t.Fatalf("DrainNode: %v", err)
	}
	nodes, err := svc.ListNodes()
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if nodes[0].Status != model.ClusterNodeDraining {
		t.Fatalf("expected draining, got %q", nodes[0].Status)
	}
	if err := svc.UndrainNode(node.UUID); err != nil {
		t.Fatalf("UndrainNode: %v", err)
	}
	nodes, err = svc.ListNodes()
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if nodes[0].Status != model.ClusterNodeReady {
		t.Fatalf("expected ready after undrain, got %q", nodes[0].Status)
	}
}

func TestClusterService_HeartbeatSchedulesPendingServer(t *testing.T) {
	svc, db := setupClusterServiceTestDB(t)
	nodeRepo := repository.NewClusterNodeRepository(db)

	if err := db.Create(&model.Domain{Name: "srv-pending", OwnerUUID: "owner-1"}).Error; err != nil {
		t.Fatalf("create domain: %v", err)
	}
	node := &model.ClusterNode{UUID: "node-w", Name: "w", Status: model.ClusterNodePending, SFUHealthy: true, MaxServers: 10, MaxRooms: 100}
	if err := nodeRepo.Create(node); err != nil {
		t.Fatalf("create node: %v", err)
	}

	healthy := true
	if _, err := svc.Heartbeat(node.UUID, cluster.HeartbeatReport{
		NodeID:     node.UUID,
		Status:     model.ClusterNodeReady,
		SFUHealthy: &healthy,
	}); err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}

	var domain model.Domain
	if err := db.First(&domain).Error; err != nil {
		t.Fatalf("load domain: %v", err)
	}
	assignments, err := svc.ListAssignments(domain.UUID)
	if err != nil {
		t.Fatalf("ListAssignments: %v", err)
	}
	if len(assignments) != 1 || assignments[0].NodeUUID != node.UUID {
		t.Fatalf("expected pending server scheduled to node-w, got %+v", assignments)
	}
}

func TestClusterService_ReapOffline(t *testing.T) {
	svc, db := setupClusterServiceTestDB(t)
	nodeRepo := repository.NewClusterNodeRepository(db)

	node := &model.ClusterNode{
		UUID:       "node-offline",
		Name:       "offline",
		Status:     model.ClusterNodeReady,
		SFUHealthy: true,
		MaxServers: 10,
		MaxRooms:   100,
		LastSeenAt: time.Now().Add(-time.Minute),
	}
	if err := nodeRepo.Create(node); err != nil {
		t.Fatalf("create node: %v", err)
	}

	if err := svc.ReapOffline(10 * time.Second); err != nil {
		t.Fatalf("ReapOffline: %v", err)
	}
	nodes, err := svc.ListNodes()
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Status != model.ClusterNodeOffline {
		t.Fatalf("expected node offline, got %+v", nodes)
	}
}

type fakeClusterNotifier struct {
	event   string
	payload interface{}
}

func (f *fakeClusterNotifier) PublishInternal(_ context.Context, event string, payload interface{}) error {
	f.event = event
	f.payload = payload
	return nil
}

func TestClusterServicePublishControlCommand(t *testing.T) {
	svc, _ := setupClusterServiceTestDB(t)
	notifier := &fakeClusterNotifier{}
	svc.SetNotifier(notifier)
	err := svc.PublishControl(cluster.ControlCommand{Command: cluster.CommandKick, NodeID: "node-a"})
	if err != nil {
		t.Fatalf("PublishControl: %v", err)
	}
	if notifier.event != cluster.EventControlCommand {
		t.Fatalf("expected control event, got %q", notifier.event)
	}
}

func TestClusterServiceReconcileAllRemovesOfflineAssignments(t *testing.T) {
	svc, db := setupClusterServiceTestDB(t)
	nodeRepo := repository.NewClusterNodeRepository(db)
	assignRepo := repository.NewServerAssignmentRepository(db)
	node := &model.ClusterNode{
		UUID: "node-old", Name: "old", Status: model.ClusterNodeOffline, SFUHealthy: false,
		MaxServers: 10, MaxRooms: 100,
	}
	if err := nodeRepo.Create(node); err != nil {
		t.Fatalf("create node: %v", err)
	}
	if err := assignRepo.Ensure("srv-1", node.UUID); err != nil {
		t.Fatalf("seed assignment: %v", err)
	}
	if err := svc.ReconcileAll(time.Hour); err != nil {
		t.Fatalf("ReconcileAll: %v", err)
	}
	assignments, err := assignRepo.ListByServer("srv-1")
	if err != nil {
		t.Fatalf("ListByServer: %v", err)
	}
	if len(assignments) != 0 {
		t.Fatalf("expected offline assignment removed, got %+v", assignments)
	}
}

func TestClusterServiceStats(t *testing.T) {
	svc, _ := setupClusterServiceTestDB(t)
	stats, err := svc.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.ReadyNodes != 0 {
		t.Fatalf("expected zero ready nodes, got %d", stats.ReadyNodes)
	}
}
