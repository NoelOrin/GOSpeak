package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"context"

	"GOSpeak/internal/bus"
	"GOSpeak/internal/cluster"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
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

func newTestDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ClusterNode{}, &model.ServerAssignment{}, &model.Domain{}); err != nil {
		t.Fatalf("migrate cluster models: %v", err)
	}
	return db, func() {
		if sqlDB, err := db.DB(); err == nil && sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
}

func TestClusterService_RegisterRejectsInvalidStatus(t *testing.T) {
	db, cleanup := newTestDB(t)
	t.Cleanup(cleanup)
	svc := NewClusterService(
		repository.NewClusterNodeRepository(db),
		repository.NewServerAssignmentRepository(db),
	)
	if _, err := svc.RegisterNode(model.ClusterNode{UUID: "node-x", Status: "bogus"}); err == nil {
		t.Fatal("expected invalid status rejection")
	}
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

func TestClusterService_RegisterNodeSecretBindsIdentity(t *testing.T) {
	svc, _ := setupClusterServiceTestDB(t)

	node := model.ClusterNode{
		UUID: "node-secret", Name: "worker-secret",
		Status: model.ClusterNodePending, Role: model.ClusterRoleWorker,
		SFUProvider: "livekit", MaxServers: 10, MaxRooms: 100,
	}
	if _, err := svc.RegisterNodeWithSecret(node, "node-secret-1"); err != nil {
		t.Fatalf("first register: %v", err)
	}

	if _, err := svc.RegisterNodeWithSecret(node, "wrong-secret"); err == nil {
		t.Fatal("expected wrong node secret to be rejected")
	}

	updated, err := svc.RegisterNodeWithSecret(node, "node-secret-1")
	if err != nil {
		t.Fatalf("register with correct secret: %v", err)
	}
	if updated.SecretHash == "" {
		t.Fatal("expected secret hash to be stored")
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

func TestClusterService_ScaleServer_InsufficientNodes(t *testing.T) {
	svc, db := setupClusterServiceTestDB(t)
	nodeRepo := repository.NewClusterNodeRepository(db)
	if err := nodeRepo.Create(&model.ClusterNode{
		UUID: "node-a", Name: "a", Status: model.ClusterNodeReady, SFUHealthy: true,
		MaxServers: 10, MaxRooms: 100, LoadPercent: 10,
	}); err != nil {
		t.Fatalf("create node: %v", err)
	}

	_, err := svc.ScaleServer("srv-1", 2, "node-a")
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) || appErr.Code != pkg.INTERNAL_ERROR {
		t.Fatalf("expected INTERNAL_ERROR for insufficient nodes, got %v", err)
	}
}

type fakeRoomMetaReader struct {
	meta bus.RoomMeta
}

func (f fakeRoomMetaReader) GetRoomMeta(_ context.Context, _ string) (bus.RoomMeta, error) {
	return f.meta, nil
}

func TestClusterService_ResolveRoomPrefersOwnerNode(t *testing.T) {
	svc, db := setupClusterServiceTestDB(t)
	nodeRepo := repository.NewClusterNodeRepository(db)
	owner := model.ClusterNode{
		UUID: "room-worker", Name: "room-worker",
		Status: model.ClusterNodeReady, SFUHealthy: true,
		AdvertiseURL: "wss://room-worker.example",
		MaxServers:   10, MaxRooms: 100,
	}
	if err := nodeRepo.Create(&owner); err != nil {
		t.Fatalf("create owner node: %v", err)
	}
	svc.SetRoomMetaStore(fakeRoomMetaReader{meta: bus.RoomMeta{OwnerNodeID: "room-worker"}})

	_, node, err := svc.ResolveRoom("domain-a", "lobby")
	if err != nil {
		t.Fatalf("ResolveRoom: %v", err)
	}
	if node == nil || node.UUID != "room-worker" {
		t.Fatalf("expected room-worker, got %+v", node)
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
	if err := svc.UndrainNode(node.UUID, 5*time.Second); err != nil {
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
	err     error
}

func (f *fakeClusterNotifier) PublishInternal(_ context.Context, event string, payload interface{}) error {
	f.event = event
	f.payload = payload
	return f.err
}

func TestClusterServicePublishControlCommand(t *testing.T) {
	svc, _ := setupClusterServiceTestDB(t)
	notifier := &fakeClusterNotifier{}
	svc.SetNotifier(notifier)
	err := svc.PublishControl(cluster.ControlCommand{Command: cluster.CommandKick, NodeID: "node-a", Room: "lobby", Identity: "alice"})
	if err != nil {
		t.Fatalf("PublishControl: %v", err)
	}
	if notifier.event != cluster.EventControlCommand {
		t.Fatalf("expected control event, got %q", notifier.event)
	}
}

func TestClusterServicePublishControlPropagatesError(t *testing.T) {
	svc, _ := setupClusterServiceTestDB(t)
	notifier := &fakeClusterNotifier{err: errors.New("publish failed")}
	svc.SetNotifier(notifier)
	err := svc.PublishControl(cluster.ControlCommand{Command: cluster.CommandKick, NodeID: "node-a", Room: "lobby", Identity: "alice"})
	if err == nil {
		t.Fatal("expected publish error")
	}
	if notifier.event != cluster.EventControlCommand {
		t.Fatalf("expected control event attempted, got %q", notifier.event)
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

func TestClusterServiceReconcileAllPreservesPendingAndDraining(t *testing.T) {
	svc, db := setupClusterServiceTestDB(t)
	nodeRepo := repository.NewClusterNodeRepository(db)
	assignRepo := repository.NewServerAssignmentRepository(db)
	nodes := []*model.ClusterNode{
		{UUID: "node-pending", Name: "pending", Status: model.ClusterNodePending, SFUHealthy: true, MaxServers: 10, MaxRooms: 100},
		{UUID: "node-draining", Name: "draining", Status: model.ClusterNodeDraining, SFUHealthy: true, MaxServers: 10, MaxRooms: 100},
	}
	for _, node := range nodes {
		if err := nodeRepo.Create(node); err != nil {
			t.Fatalf("create node: %v", err)
		}
		if err := assignRepo.Ensure("srv-1", node.UUID); err != nil {
			t.Fatalf("seed assignment: %v", err)
		}
	}
	if err := svc.ReconcileAll(time.Hour); err != nil {
		t.Fatalf("ReconcileAll: %v", err)
	}
	assignments, err := assignRepo.ListByServer("srv-1")
	if err != nil {
		t.Fatalf("ListByServer: %v", err)
	}
	if len(assignments) != 2 {
		t.Fatalf("expected pending/draining assignments preserved, got %+v", assignments)
	}
}

func TestClusterServiceReconcileAllRemovesUnhealthyAssignments(t *testing.T) {
	svc, db := setupClusterServiceTestDB(t)
	nodeRepo := repository.NewClusterNodeRepository(db)
	assignRepo := repository.NewServerAssignmentRepository(db)
	node := &model.ClusterNode{UUID: "node-unhealthy", Name: "unhealthy", Status: model.ClusterNodeUnhealthy, SFUHealthy: false, MaxServers: 10, MaxRooms: 100}
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
		t.Fatalf("expected unhealthy assignment removed, got %+v", assignments)
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

func TestClusterServiceAutoScaleNoReadyNodes(t *testing.T) {
	svc, _ := setupClusterServiceTestDB(t)
	if err := svc.AutoScale("srv-1", 3); err != nil {
		t.Fatalf("AutoScale: %v", err)
	}
	assignments, err := svc.ListAssignments("srv-1")
	if err != nil {
		t.Fatalf("ListAssignments: %v", err)
	}
	if len(assignments) != 0 {
		t.Fatalf("expected no scaling without ready nodes, got %+v", assignments)
	}
}

func TestClusterServiceMarkServerAssignmentsDraining(t *testing.T) {
	svc, db := setupClusterServiceTestDB(t)
	nodeRepo := repository.NewClusterNodeRepository(db)
	assignRepo := repository.NewServerAssignmentRepository(db)
	node := &model.ClusterNode{UUID: "node-a", Name: "a", Status: model.ClusterNodeReady, SFUHealthy: true, MaxServers: 10, MaxRooms: 100}
	if err := nodeRepo.Create(node); err != nil {
		t.Fatalf("create node: %v", err)
	}
	if err := assignRepo.Ensure("srv-1", node.UUID); err != nil {
		t.Fatalf("seed assignment: %v", err)
	}
	if err := svc.MarkServerAssignmentsDraining("srv-1"); err != nil {
		t.Fatalf("MarkServerAssignmentsDraining: %v", err)
	}
	assignments, err := assignRepo.ListByServer("srv-1")
	if err != nil {
		t.Fatalf("ListByServer: %v", err)
	}
	if len(assignments) != 1 || assignments[0].Status != model.ServerAssignmentDraining {
		t.Fatalf("expected draining assignment, got %+v", assignments)
	}
}

func TestReconcile_ReportsErrors(t *testing.T) {
	svc, db := setupClusterServiceTestDB(t)
	nodeRepo := repository.NewClusterNodeRepository(db)
	node := &model.ClusterNode{UUID: "node-w", Name: "w", Status: model.ClusterNodeReady, SFUHealthy: true, MaxServers: 10, MaxRooms: 100}
	if err := nodeRepo.Create(node); err != nil {
		t.Fatalf("create node: %v", err)
	}
	if err := db.Create(&model.Domain{Name: "srv-pending", OwnerUUID: "owner-1"}).Error; err != nil {
		t.Fatalf("create domain: %v", err)
	}
	svc.SetNotifier(&fakeClusterNotifier{err: errors.New("publish failed")})

	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	svc.reconcilePendingServers(node.UUID)

	if !strings.Contains(buf.String(), "[cluster] reconcile scale failed server=") {
		t.Fatalf("expected reconcile scale failure to be logged, got %q", buf.String())
	}
}

func TestClusterServicePublishControlUsesControlQueue(t *testing.T) {
	svc, _ := setupClusterServiceTestDB(t)
	queue := &fakeQueue{}
	svc.SetControlQueue(queue)
	err := svc.PublishControl(cluster.ControlCommand{Command: cluster.CommandKick, Room: "lobby", Identity: "alice"})
	if err != nil {
		t.Fatalf("PublishControl: %v", err)
	}
	if len(queue.jobs) != 1 {
		t.Fatalf("expected 1 queued control job, got %d", len(queue.jobs))
	}
	job := queue.jobs[0]
	if job.Type != "cluster.control" {
		t.Fatalf("expected cluster.control job, got %s", job.Type)
	}
	var cmd cluster.ControlCommand
	if err := json.Unmarshal(job.Payload, &cmd); err != nil {
		t.Fatalf("unmarshal control payload: %v", err)
	}
	if cmd.Command != cluster.CommandKick || cmd.Room != "lobby" || cmd.Identity != "alice" {
		t.Fatalf("unexpected control payload: %+v", cmd)
	}
}

func TestClusterServiceUndrainRejectsStaleNode(t *testing.T) {
	svc, db := setupClusterServiceTestDB(t)
	nodeRepo := repository.NewClusterNodeRepository(db)
	node := &model.ClusterNode{
		UUID: "node-stale", Name: "stale", Status: model.ClusterNodeDraining,
		SFUHealthy: true, MaxServers: 10, MaxRooms: 100,
		LastSeenAt: time.Now().Add(-time.Minute),
	}
	if err := nodeRepo.Create(node); err != nil {
		t.Fatalf("create node: %v", err)
	}
	if err := svc.UndrainNode(node.UUID, 10*time.Second); err == nil {
		t.Fatal("expected stale undrain rejection")
	}
}

func TestClusterServiceHeartbeatMarksUnhealthy(t *testing.T) {
	svc, _ := setupClusterServiceTestDB(t)
	node, err := svc.RegisterNode(model.ClusterNode{
		UUID: "node-unhealthy-hb", Name: "uh", Role: model.ClusterRoleWorker,
		MaxServers: 10, MaxRooms: 100,
	})
	if err != nil {
		t.Fatalf("RegisterNode: %v", err)
	}
	healthy := false
	updated, err := svc.Heartbeat(node.UUID, cluster.HeartbeatReport{
		NodeID: node.UUID, Status: model.ClusterNodeReady, SFUHealthy: &healthy,
	})
	if err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}
	if updated.Status != model.ClusterNodeUnhealthy {
		t.Fatalf("expected unhealthy, got %q", updated.Status)
	}
}

func TestClusterServiceReapOfflineCoversDrainingAndUnhealthy(t *testing.T) {
	svc, db := setupClusterServiceTestDB(t)
	nodeRepo := repository.NewClusterNodeRepository(db)
	for _, status := range []string{model.ClusterNodeDraining, model.ClusterNodeUnhealthy} {
		node := &model.ClusterNode{
			UUID: "node-" + status, Name: status, Status: status,
			SFUHealthy: true, MaxServers: 10, MaxRooms: 100,
			LastSeenAt: time.Now().Add(-time.Minute),
		}
		if err := nodeRepo.Create(node); err != nil {
			t.Fatalf("create node %s: %v", status, err)
		}
	}
	if err := svc.ReapOffline(10 * time.Second); err != nil {
		t.Fatalf("ReapOffline: %v", err)
	}
	for _, status := range []string{model.ClusterNodeDraining, model.ClusterNodeUnhealthy} {
		node, err := nodeRepo.GetByUUID("node-" + status)
		if err != nil {
			t.Fatalf("GetByUUID %s: %v", status, err)
		}
		if node.Status != model.ClusterNodeOffline {
			t.Fatalf("expected node-%s offline, got %q", status, node.Status)
		}
	}
}

func TestClusterServiceConcurrentScaleKeepsSingleReplica(t *testing.T) {
	svc, db := setupClusterServiceTestDB(t)
	nodeRepo := repository.NewClusterNodeRepository(db)
	nodes := []model.ClusterNode{
		{UUID: "node-a", Name: "a", Status: model.ClusterNodeReady, SFUHealthy: true, MaxServers: 10, MaxRooms: 100},
		{UUID: "node-b", Name: "b", Status: model.ClusterNodeReady, SFUHealthy: true, MaxServers: 10, MaxRooms: 100},
		{UUID: "node-c", Name: "c", Status: model.ClusterNodeReady, SFUHealthy: true, MaxServers: 10, MaxRooms: 100},
	}
	for i := range nodes {
		if err := nodeRepo.Create(&nodes[i]); err != nil {
			t.Fatalf("create node %d: %v", i, err)
		}
	}
	preferred := []string{"node-a", "node-b", "node-c"}
	var wg sync.WaitGroup
	for i := 0; i < 18; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = svc.ScaleServer("srv-concurrent", 1, preferred[i%len(preferred)])
		}(i)
	}
	wg.Wait()
	assignments, err := svc.ListAssignments("srv-concurrent")
	if err != nil {
		t.Fatalf("ListAssignments: %v", err)
	}
	if len(assignments) != 1 {
		t.Fatalf("expected exactly 1 replica, got %+v", assignments)
	}
}

func TestClusterServiceReconcileAllDoesNotReapStaleReadyNode(t *testing.T) {
	svc, db := setupClusterServiceTestDB(t)
	nodeRepo := repository.NewClusterNodeRepository(db)
	node := &model.ClusterNode{
		UUID: "node-stale-ready", Name: "stale-ready", Status: model.ClusterNodeReady,
		SFUHealthy: true, MaxServers: 10, MaxRooms: 100,
		LastSeenAt: time.Now().Add(-time.Hour),
	}
	if err := nodeRepo.Create(node); err != nil {
		t.Fatalf("create node: %v", err)
	}
	// Agent 重启场景：ReconcileAll 不得把心跳过期但尚未重新上报的在线节点标离线。
	if err := svc.ReconcileAll(10 * time.Second); err != nil {
		t.Fatalf("ReconcileAll: %v", err)
	}
	got, err := nodeRepo.GetByUUID(node.UUID)
	if err != nil {
		t.Fatalf("GetByUUID: %v", err)
	}
	if got.Status != model.ClusterNodeReady {
		t.Fatalf("ReconcileAll must not reap nodes; got %q", got.Status)
	}
}

func TestClusterNodeSecretGuards(t *testing.T) {
	svc, db := setupClusterServiceTestDB(t)
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil && sqlDB != nil {
			_ = sqlDB.Close()
		}
	})

	node, err := svc.RegisterNodeParams("node-sec", "node-sec", "h", "http://h", model.ClusterRoleWorker, model.ClusterNodeReady, "livekit", 10, 100, nil, "s3cret")
	if err != nil {
		t.Fatalf("register with secret: %v", err)
	}

	// 无 secret 的节点可直接操作（向后兼容）。
	plain, err := svc.RegisterNodeParams("node-plain", "node-plain", "h", "http://h", model.ClusterRoleWorker, model.ClusterNodeReady, "livekit", 10, 100, nil, "")
	if err != nil {
		t.Fatalf("register plain: %v", err)
	}
	if _, err := svc.Heartbeat(plain.UUID, cluster.HeartbeatReport{NodeID: plain.UUID}); err != nil {
		t.Fatalf("plain heartbeat should pass: %v", err)
	}
	if err := svc.DeregisterNode(plain.UUID, ""); err != nil {
		t.Fatalf("plain deregister should pass: %v", err)
	}

	// 有 secret 的节点：错误/缺失 secret 必须被拒。
	if _, err := svc.Heartbeat(node.UUID, cluster.HeartbeatReport{NodeID: node.UUID}); err == nil {
		t.Fatal("heartbeat without secret should fail")
	}
	if _, err := svc.Heartbeat(node.UUID, cluster.HeartbeatReport{NodeID: node.UUID, NodeSecret: "wrong"}); err == nil {
		t.Fatal("heartbeat with wrong secret should fail")
	}
	if err := svc.DeregisterNode(node.UUID, ""); err == nil {
		t.Fatal("deregister without secret should fail")
	}

	// 正确 secret 放行。
	if _, err := svc.Heartbeat(node.UUID, cluster.HeartbeatReport{NodeID: node.UUID, NodeSecret: "s3cret"}); err != nil {
		t.Fatalf("heartbeat with correct secret: %v", err)
	}
	if err := svc.DeregisterNode(node.UUID, "s3cret"); err != nil {
		t.Fatalf("deregister with correct secret: %v", err)
	}
}
