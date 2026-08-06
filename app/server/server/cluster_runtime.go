package server

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"GOSpeak/internal/cluster"
	"GOSpeak/internal/config"
	"GOSpeak/internal/logger"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/service"
	"GOSpeak/internal/sfu"
	"GOSpeak/internal/signal"

	"github.com/nats-io/nats.go"
)

const agentLeaderAcquireTimeout = 5 * time.Second

// acquireAgentLeader 尝试通过 NATS JetStream KV 抢占 Agent 主锁。
func acquireAgentLeader(ctx context.Context, nc *nats.Conn, prefix, instanceID string) (*cluster.NATSLeaderLock, bool, error) {
	if nc == nil {
		return nil, false, nil
	}
	if strings.TrimSpace(prefix) == "" {
		prefix = "gospeak"
	}
	js, err := nc.JetStream()
	if err != nil {
		return nil, false, err
	}
	lock, err := cluster.OpenLeaderLock(js, prefix)
	if err != nil {
		return nil, false, err
	}
	ok, err := lock.TryAcquire(ctx, instanceID)
	return lock, ok, err
}

// startDegradedLocalWorkerRuntime 在主锁不可用时把节点作为本地 worker 数据面运行，
// 不携带 serverRepo，避免心跳触发控制面调度写操作。
func startDegradedLocalWorkerRuntime(cfg *config.Config, hub *signal.Hub, instanceID string, sfuProvider sfu.Provider) (string, func(), error) {
	workerSvc := service.NewClusterService(
		repository.NewClusterNodeRepository(repository.DB),
		repository.NewServerAssignmentRepository(repository.DB),
	)
	return startLocalClusterRuntime(cfg, workerSvc, hub, instanceID, sfuProvider)
}

func startLocalClusterRuntime(cfg *config.Config, clusterSvc *service.ClusterService, hub *signal.Hub, instanceID string, sfuProvider sfu.Provider) (string, func(), error) {
	node, err := clusterSvc.EnsureLocalNode(cfg, instanceID)
	if err != nil {
		return "", func() {}, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	interval := cfg.ClusterHeartbeatIntervalDuration()
	timeout := cfg.ClusterHeartbeatTimeoutDuration()

	report := collectClusterReport(cfg, node.UUID, hub, probeSFUHealth(context.Background(), sfuProvider))
	if _, err := clusterSvc.Heartbeat(node.UUID, report); err != nil {
		logger.WithComponent("Cluster").Warnf("local node initial heartbeat failed: %v", err)
	}

	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			report := collectClusterReport(cfg, node.UUID, hub, probeSFUHealth(ctx, sfuProvider))
			if _, err := clusterSvc.Heartbeat(node.UUID, report); err != nil {
				logger.WithComponent("Cluster").Warnf("local node heartbeat failed: %v", err)
			}
		}
	}()

	if cfg.IsAgent() {
		go func() {
			reaper := time.NewTicker(timeout)
			defer reaper.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-reaper.C:
					if err := clusterSvc.ReapOffline(timeout); err != nil {
						logger.WithComponent("Cluster").Warnf("reap offline nodes failed: %v", err)
					}
				}
			}
		}()
	}

	if cfg.IsAgent() {
		if err := clusterSvc.ReconcileAll(timeout); err != nil {
			logger.WithComponent("Cluster").Warnf("reconcile failed: %v", err)
		}
	}

	stop := func() {
		cancel()
		<-done
		if err := clusterSvc.DeregisterNode(node.UUID); err != nil {
			logger.WithComponent("Cluster").Debugf("local node deregister failed: %v", err)
		}
	}
	return node.UUID, stop, nil
}

func startAgentClusterRuntime(cfg *config.Config, clusterSvc *service.ClusterService) (func(), error) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	timeout := cfg.ClusterHeartbeatTimeoutDuration()

	go func() {
		defer close(done)
		reaper := time.NewTicker(timeout)
		defer reaper.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-reaper.C:
			}
			if err := clusterSvc.ReapOffline(timeout); err != nil {
				logger.WithComponent("Cluster").Warnf("reap offline nodes failed: %v", err)
			}
		}
	}()

	if err := clusterSvc.ReconcileAll(timeout); err != nil {
		logger.WithComponent("Cluster").Warnf("reconcile failed: %v", err)
	}

	stop := func() {
		cancel()
		<-done
	}
	return stop, nil
}

func startWorkerClusterRuntime(cfg *config.Config, hub *signal.Hub, sfuProvider sfu.Provider) (string, func(), error) {
	nodeID := strings.TrimSpace(cfg.ClusterNodeID)
	if nodeID == "" {
		host, _ := os.Hostname()
		nodeID = "node-" + sanitizeWorkerID(host) + "-" + strconv.Itoa(os.Getpid())
	}
	req := cluster.RegisterRequest{
		UUID:         nodeID,
		Name:         nodeID,
		Host:         hostnameOrDefault(),
		AdvertiseURL: workerAdvertiseURL(cfg),
		Role:         model.ClusterRoleWorker,
		SFUProvider:  cfg.SFUProvider,
		MaxServers:   cfg.ClusterMaxServers,
		MaxRooms:     cfg.ClusterMaxRooms,
		Labels:       cluster.ParseLabels(cfg.ClusterLabels),
	}
	client := cluster.NewAgentClient(cfg.ClusterAgentURL, cfg.ClusterAgentToken)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	interval := cfg.ClusterHeartbeatIntervalDuration()
	registered := false

	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			if !registered {
				if err := client.Register(ctx, req); err != nil {
					logger.WithComponent("Cluster").Warnf("worker register failed: %v", err)
					continue
				}
				registered = true
				logger.WithComponent("Cluster").Infof("worker registered node=%s", nodeID)
			}
			report := collectClusterReport(cfg, nodeID, hub, probeSFUHealth(ctx, sfuProvider))
			if err := client.Heartbeat(ctx, report); err != nil {
				logger.WithComponent("Cluster").Warnf("worker heartbeat failed: %v", err)
				if errors.Is(err, cluster.ErrClusterNodeNotFound) {
					// Agent 侧节点记录丢失（重建/清理）：回到注册流程。
					registered = false
				}
			}
		}
	}()

	stop := func() {
		cancel()
		<-done
		deregCtx, deregCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer deregCancel()
		if err := client.Deregister(deregCtx, nodeID); err != nil {
			logger.WithComponent("Cluster").Debugf("worker deregister failed: %v", err)
		}
	}
	return nodeID, stop, nil
}

func collectClusterReport(cfg *config.Config, nodeID string, hub *signal.Hub, sfuHealthy bool) cluster.HeartbeatReport {
	rooms := 0
	connections := 0
	if hub != nil {
		stats := hub.GetStats()
		rooms = stats.RoomCount
		connections = stats.ParticipantCount
	}
	load := 0.0
	if cfg.ClusterMaxRooms > 0 {
		load = float64(rooms) / float64(cfg.ClusterMaxRooms) * 100
	}
	status := model.ClusterNodeReady
	if load >= 80 {
		status = model.ClusterNodeBusy
	}
	if !sfuHealthy {
		status = model.ClusterNodeUnhealthy
	}
	return cluster.HeartbeatReport{
		NodeID:       nodeID,
		Status:       status,
		AdvertiseURL: cfg.ClusterAdvertiseURL,
		Rooms:        rooms,
		Connections:  connections,
		LoadPercent:  load,
		SFUHealthy:   &sfuHealthy,
	}
}

// probeSFUHealth does a lightweight provider probe. Providers without a
// real-time management list (ErrSFUNotSupported) are treated as healthy.
// Provider clients are expected to bound their own call latency.
func probeSFUHealth(_ context.Context, provider sfu.Provider) bool {
	if provider == nil {
		return true
	}
	_, err := provider.ListRooms()
	return err == nil || errors.Is(err, pkg.ErrSFUNotSupported)
}

func hostnameOrDefault() string {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		return "localhost"
	}
	return host
}

func sanitizeWorkerID(s string) string {
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, ":", "-")
	if s == "" {
		return "worker"
	}
	return s
}

func workerAdvertiseURL(cfg *config.Config) string {
	if strings.TrimSpace(cfg.ClusterAdvertiseURL) != "" {
		return cfg.ClusterAdvertiseURL
	}
	port := strings.TrimSpace(cfg.ServerPort)
	if port == "" {
		port = "8998"
	}
	return "http://" + hostnameOrDefault() + ":" + port
}
