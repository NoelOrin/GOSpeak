package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"GOSpeak/internal/cluster"
	"GOSpeak/internal/config"
	"GOSpeak/internal/handler"
	"GOSpeak/internal/logger"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/service"
	"GOSpeak/internal/sfu"
	"GOSpeak/internal/signal"

	"github.com/nats-io/nats.go"
	"gorm.io/gorm"
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
func startDegradedLocalWorkerRuntime(db *gorm.DB, cfg *config.Config, hub *signal.Hub, instanceID string, sfuProvider sfu.Provider) (string, func(), error) {
	workerSvc := service.NewClusterService(
		repository.NewClusterNodeRepository(db),
		repository.NewServerAssignmentRepository(db),
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
	client := cluster.NewAgentClient(cfg.ClusterAgentURL, cfg.ClusterAgentToken, cfg.ClusterNodeSecret)

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
	lagMs := int64(0)
	lagDegraded := false
	if lag, lagErr := repository.ReplicaLag(); lagErr == nil {
		lagMs = lag.Milliseconds()
		threshold := cfg.ReplicaLagThresholdDuration()
		lagDegraded = threshold > 0 && lag > threshold
	} else {
		lagMs = -1
	}
	return cluster.HeartbeatReport{
		NodeID:               nodeID,
		Status:               status,
		AdvertiseURL:         cfg.ClusterAdvertiseURL,
		Rooms:                rooms,
		Connections:          connections,
		LoadPercent:          load,
		SFUHealthy:           &sfuHealthy,
		DBReplicaLagMs:       lagMs,
		DBReplicaLagDegraded: lagDegraded,
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

// startClusterRuntimes 选择并启动当前节点的 cluster runtime（leader lock / all / worker / agent）。
// 返回控制面 handler、清理函数与节点身份，供组合根接线与优雅关闭使用。
func startClusterRuntimes(cfg *config.Config, db *gorm.DB, natsConn *nats.Conn, instanceID string, clusterSvc *service.ClusterService, hub *signal.Hub, sfuProvider sfu.Provider) (*handler.ClusterHandler, func(), *cluster.NATSLeaderLock, func(), *service.LeaderFenceService, string, bool, error) {
	var clusterHandler *handler.ClusterHandler
	var clusterStop func()
	var agentLeaderLock *cluster.NATSLeaderLock
	var stopLeaderRenew func()
	var leaderFence *service.LeaderFenceService
	localNodeUUID := ""
	degradedToWorker := false
	var err error
	if cfg.IsAgent() {
		acquireCtx, cancel := context.WithTimeout(context.Background(), agentLeaderAcquireTimeout)
		leaderLock, leader, lockErr := acquireAgentLeader(acquireCtx, natsConn, cfg.NATSSubjectPrefix, instanceID)
		cancel()
		switch {
		case lockErr != nil:
			logger.WithComponent("Cluster").Warnf("agent leader lock unavailable; degraded-to-worker instance=%s role=%s err=%v", instanceID, cfg.ClusterRole, lockErr)
			degradedToWorker = true
		case !leader:
			logger.WithComponent("Cluster").Warnf("agent leader lock held by another instance; degraded-to-worker instance=%s role=%s", instanceID, cfg.ClusterRole)
			degradedToWorker = true
		default:
			agentLeaderLock = leaderLock
			leaderFence = service.NewLeaderFenceService(repository.NewClusterFenceRepository(db), instanceID)
			if fenceErr := leaderFence.Acquire(); fenceErr != nil {
				logger.WithComponent("Cluster").Errorf("acquire db leader fence failed instance=%s err=%v", instanceID, fenceErr)
				_ = leaderLock.Release(instanceID)
				return nil, nil, nil, nil, nil, "", false, fmt.Errorf("acquire db leader fence: %w", fenceErr)
			}
			renewCtx, renewCancel := context.WithCancel(context.Background())
			renewInterval := cfg.ClusterHeartbeatIntervalDuration() / 2
			if renewInterval > 2*time.Second {
				renewInterval = 2 * time.Second
			}
			renewDone, renewLost := leaderLock.RenewLoop(renewCtx, instanceID, renewInterval)
			go func() {
				select {
				case <-renewLost:
					// 锁丢失意味着另一个 Agent 可能已接管：继续保留写面会形成双写，
					// 直接触发优雅退出，由编排重启并重新竞争。
					if leaderFence != nil {
						leaderFence.Deactivate()
					}
					logger.WithComponent("Cluster").Errorf("agent leader lock lost; shutting down to avoid split brain instance=%s", instanceID)
					terminateSelf()
				case <-renewDone:
				}
			}()
			stopLeaderRenew = func() {
				renewCancel()
				<-renewDone
			}
			logger.WithComponent("Cluster").Infof("agent leader lock acquired instance=%s role=%s", instanceID, cfg.ClusterRole)
		}
	}
	if degradedToWorker {
		cfg.ClusterRole = model.ClusterRoleWorker
	}
	if cfg.IsAgent() {
		clusterHandler = handler.NewClusterHandler(clusterSvc, cfg)
	}
	switch cfg.ClusterRole {
	case "all":
		localNodeUUID, clusterStop, err = startLocalClusterRuntime(cfg, clusterSvc, hub, instanceID, sfuProvider)
	case "worker":
		if degradedToWorker && (cfg.ClusterAgentURL == "" || cfg.ClusterAgentToken == "") {
			localNodeUUID, clusterStop, err = startDegradedLocalWorkerRuntime(db, cfg, hub, instanceID, sfuProvider)
		} else {
			localNodeUUID, clusterStop, err = startWorkerClusterRuntime(cfg, hub, sfuProvider)
		}
	case "agent":
		clusterStop, err = startAgentClusterRuntime(cfg, clusterSvc)
	}
	if err != nil {
		return nil, nil, nil, nil, nil, "", false, fmt.Errorf("failed to start cluster runtime: %w", err)
	}
	return clusterHandler, clusterStop, agentLeaderLock, stopLeaderRenew, leaderFence, localNodeUUID, degradedToWorker, nil
}
