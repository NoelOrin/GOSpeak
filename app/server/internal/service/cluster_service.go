package service

import (
	"fmt"
	"os"
	"strings"
	"time"

	"context"

	"GOSpeak/internal/cluster"
	"GOSpeak/internal/config"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"gorm.io/gorm"
)

var (
	ErrClusterNodeNotFound     = pkg.NewAppError(pkg.NOT_FOUND, "cluster node not found")
	ErrClusterServerUnassigned = pkg.NewAppError(pkg.NOT_FOUND, "no healthy worker assigned to server")
)

// ClusterService 是 Agent 控制面的节点与 Server 分配服务。

// ClusterService 是 Agent 控制面的节点与 Server 分配服务。
type ClusterService struct {
	nodeRepo   *repository.ClusterNodeRepository
	assignRepo *repository.ServerAssignmentRepository
	notifier   ClusterNotifier
	serverRepo *repository.DomainRepository
}

func NewClusterService(
	nodeRepo *repository.ClusterNodeRepository,
	assignRepo *repository.ServerAssignmentRepository,
) *ClusterService {
	return &ClusterService{nodeRepo: nodeRepo, assignRepo: assignRepo}
}

// ClusterNotifier 发布跨实例控制面事件。

// ClusterNotifier 发布跨实例控制面事件。
type ClusterNotifier interface {
	PublishInternal(ctx context.Context, event string, payload interface{}) error
}

// SetNotifier 注入 NATS/EventBus internal 发布器。

// SetNotifier 注入 NATS/EventBus internal 发布器。
func (s *ClusterService) SetNotifier(n ClusterNotifier) {
	s.notifier = n
}

// SetServerRepo 注入 Domain 仓库，用于节点上线后补调度未分配的 Server。

// SetServerRepo 注入 Domain 仓库，用于节点上线后补调度未分配的 Server。
func (s *ClusterService) SetServerRepo(repo *repository.DomainRepository) {
	s.serverRepo = repo
}

// EnsureLocalNode 在 all/agent 模式下注册当前进程对应的本地节点。

// EnsureLocalNode 在 all/agent 模式下注册当前进程对应的本地节点。
func (s *ClusterService) EnsureLocalNode(cfg *config.Config, instanceID string) (*model.ClusterNode, error) {
	if cfg == nil {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "config is nil")
	}
	nodeID := strings.TrimSpace(cfg.ClusterNodeID)
	if nodeID == "" {
		nodeID = "node-" + sanitizeClusterID(instanceID)
	}
	host, _ := os.Hostname()
	if host == "" {
		host = "localhost"
	}

	node := &model.ClusterNode{
		UUID:         nodeID,
		Name:         instanceID,
		Host:         host,
		AdvertiseURL: localAdvertiseURL(cfg, host),
		Role:         cfg.ClusterRole,
		Status:       model.ClusterNodePending,
		SFUProvider:  cfg.SFUProvider,
		MaxServers:   cfg.ClusterMaxServers,
		MaxRooms:     cfg.ClusterMaxRooms,
		SFUHealthy:   true,
		LastSeenAt:   time.Now(),
	}
	node.SetLabels(cluster.ParseLabels(cfg.ClusterLabels))

	existing, err := s.nodeRepo.GetByUUID(nodeID)
	if err == nil {
		existing.Host = host
		existing.AdvertiseURL = localAdvertiseURL(cfg, host)
		existing.Role = cfg.ClusterRole
		existing.SFUProvider = cfg.SFUProvider
		existing.MaxServers = cfg.ClusterMaxServers
		existing.MaxRooms = cfg.ClusterMaxRooms
		existing.SFUHealthy = true
		existing.SetLabels(cluster.ParseLabels(cfg.ClusterLabels))
		if existing.Status == "" {
			existing.Status = model.ClusterNodePending
		}
		if err := s.nodeRepo.Update(existing); err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		if err := s.publishClusterEvent(cluster.EventNodeRegistered, map[string]interface{}{
			"node_id": existing.UUID, "status": existing.Status, "advertise_url": existing.AdvertiseURL,
		}); err != nil {
			return nil, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "publish node registered event failed")
		}
		return existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := s.nodeRepo.Create(node); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return node, nil
}

// RegisterNode 创建或更新 Worker 节点。

// RegisterNode 创建或更新 Worker 节点。
func (s *ClusterService) RegisterNode(req model.ClusterNode) (*model.ClusterNode, error) {
	if strings.TrimSpace(req.UUID) == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "node uuid is required")
	}
	if req.Name == "" {
		req.Name = req.UUID
	}
	if req.Status == "" {
		req.Status = model.ClusterNodePending
	}
	if req.MaxServers <= 0 {
		req.MaxServers = 100
	}
	if req.MaxRooms <= 0 {
		req.MaxRooms = 1000
	}

	existing, err := s.nodeRepo.GetByUUID(req.UUID)
	if err == nil {
		existing.Name = req.Name
		existing.Host = req.Host
		existing.AdvertiseURL = req.AdvertiseURL
		existing.Role = req.Role
		existing.SFUProvider = req.SFUProvider
		existing.MaxServers = req.MaxServers
		existing.MaxRooms = req.MaxRooms
		if existing.Status != model.ClusterNodeDraining {
			existing.Status = req.Status
		}
		existing.LastSeenAt = time.Now()
		existing.SetLabels(req.LabelMap())
		if err := s.nodeRepo.Update(existing); err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		return existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	req.LastSeenAt = time.Now()
	if err := s.nodeRepo.Create(&req); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := s.publishClusterEvent(cluster.EventNodeRegistered, map[string]interface{}{
		"node_id": req.UUID, "status": req.Status, "advertise_url": req.AdvertiseURL,
	}); err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "publish node registered event failed")
	}
	return &req, nil
}

// Heartbeat 更新节点运行时快照并刷新 last_seen。

// Heartbeat 更新节点运行时快照并刷新 last_seen。
func (s *ClusterService) Heartbeat(nodeID string, report cluster.HeartbeatReport) (*model.ClusterNode, error) {
	node, err := s.nodeRepo.GetByUUID(nodeID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrClusterNodeNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	status := report.Status
	if status == "" {
		status = model.ClusterNodeReady
	}
	switch status {
	case model.ClusterNodeReady, model.ClusterNodeBusy, model.ClusterNodeUnhealthy:
	default:
		status = model.ClusterNodeReady
	}
	// draining 由控制面操作驱动，不能被心跳覆盖。
	if node.Status == model.ClusterNodeDraining {
		status = model.ClusterNodeDraining
	}
	node.Status = status
	if report.AdvertiseURL != "" {
		node.AdvertiseURL = report.AdvertiseURL
	}
	node.Rooms = report.Rooms
	node.Connections = report.Connections
	node.LoadPercent = report.LoadPercent
	if report.SFUHealthy != nil {
		node.SFUHealthy = *report.SFUHealthy
	}
	if report.ServingServers > 0 {
		node.ServingServers = report.ServingServers
	} else if count, countErr := s.assignRepo.CountByNode(nodeID); countErr == nil {
		node.ServingServers = int(count)
	}
	node.LastSeenAt = time.Now()

	if err := s.nodeRepo.Update(node); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := s.publishClusterEvent(cluster.EventNodeHeartbeat, map[string]interface{}{
		"node_id": node.UUID, "status": node.Status, "rooms": node.Rooms, "connections": node.Connections,
	}); err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "publish node heartbeat event failed")
	}
	s.reconcilePendingServers(node.UUID)
	return node, nil
}

// DeregisterNode 注销节点并标记为 offline。

// DeregisterNode 注销节点并标记为 offline。
func (s *ClusterService) DeregisterNode(nodeID string) error {
	node, err := s.nodeRepo.GetByUUID(nodeID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrClusterNodeNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	node.Status = model.ClusterNodeOffline
	node.LastSeenAt = time.Now()
	if err := s.nodeRepo.Update(node); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := s.publishClusterEvent(cluster.EventNodeDeregistered, map[string]interface{}{
		"node_id": node.UUID, "status": node.Status,
	}); err != nil {
		return pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "publish node deregistered event failed")
	}
	return nil
}

// DrainNode 标记节点 draining，停止新的 Server 分配。

// DrainNode 标记节点 draining，停止新的 Server 分配。
func (s *ClusterService) DrainNode(nodeID string) error {
	node, err := s.nodeRepo.GetByUUID(nodeID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrClusterNodeNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	node.Status = model.ClusterNodeDraining
	if err := s.nodeRepo.Update(node); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := s.publishClusterEvent(cluster.EventNodeDraining, map[string]interface{}{
		"node_id": node.UUID, "status": node.Status,
	}); err != nil {
		return pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "publish node draining event failed")
	}
	return nil
}

// UndrainNode 恢复节点为 ready，允许继续调度。

// UndrainNode 恢复节点为 ready，允许继续调度。
func (s *ClusterService) UndrainNode(nodeID string) error {
	node, err := s.nodeRepo.GetByUUID(nodeID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrClusterNodeNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	node.Status = model.ClusterNodeReady
	if err := s.nodeRepo.Update(node); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := s.publishClusterEvent(cluster.EventNodeUndrained, map[string]interface{}{
		"node_id": node.UUID, "status": node.Status,
	}); err != nil {
		return pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "publish node undrained event failed")
	}
	return nil
}

// ListNodes 返回全部节点记录。

// ListNodes 返回全部节点记录。
func (s *ClusterService) ListNodes() ([]model.ClusterNode, error) {
	nodes, err := s.nodeRepo.List()
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nodes, nil
}

// ReapOffline 将超过 timeout 未心跳的节点标记为 offline。

// ReapOffline 将超过 timeout 未心跳的节点标记为 offline。
func (s *ClusterService) ReapOffline(timeout time.Duration) error {
	if timeout <= 0 {
		return nil
	}
	if err := s.nodeRepo.MarkOfflineBefore(time.Now().Add(-timeout)); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// ScaleServer 调整 Server（Domain）的实例副本数。
// preferredNode 通常传本地 all 节点 UUID，让单机模式优先把 Server 分配回本节点。

func sanitizeClusterID(s string) string {
	replacer := strings.NewReplacer(" ", "-", "/", "-", ":", "-")
	s = replacer.Replace(strings.TrimSpace(s))
	if s == "" {
		return fmt.Sprintf("local-%d", os.Getpid())
	}
	return s
}

func localAdvertiseURL(cfg *config.Config, host string) string {
	if strings.TrimSpace(cfg.ClusterAdvertiseURL) != "" {
		return cfg.ClusterAdvertiseURL
	}
	port := strings.TrimSpace(cfg.ServerPort)
	if port == "" {
		port = "8998"
	}
	if host == "" {
		host = "localhost"
	}
	return "http://" + host + ":" + port
}

func (s *ClusterService) publishClusterEvent(event string, payload interface{}) error {
	if s.notifier == nil {
		return nil
	}
	return s.notifier.PublishInternal(context.Background(), event, payload)
}
