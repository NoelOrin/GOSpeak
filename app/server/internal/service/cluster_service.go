package service

import (
	"fmt"
	"os"
	"sort"
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
type ClusterNotifier interface {
	PublishInternal(ctx context.Context, event string, payload interface{}) error
}

// SetNotifier 注入 NATS/EventBus internal 发布器。
func (s *ClusterService) SetNotifier(n ClusterNotifier) {
	s.notifier = n
}

// SetServerRepo 注入 Domain 仓库，用于节点上线后补调度未分配的 Server。
func (s *ClusterService) SetServerRepo(repo *repository.DomainRepository) {
	s.serverRepo = repo
}

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
		s.publishClusterEvent(cluster.EventNodeRegistered, map[string]interface{}{
			"node_id": existing.UUID, "status": existing.Status, "advertise_url": existing.AdvertiseURL,
		})
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
	s.publishClusterEvent(cluster.EventNodeRegistered, map[string]interface{}{
		"node_id": req.UUID, "status": req.Status, "advertise_url": req.AdvertiseURL,
	})
	return &req, nil
}

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
	s.publishClusterEvent(cluster.EventNodeHeartbeat, map[string]interface{}{
		"node_id": node.UUID, "status": node.Status, "rooms": node.Rooms, "connections": node.Connections,
	})
	s.reconcilePendingServers(node.UUID)
	return node, nil
}

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
	s.publishClusterEvent(cluster.EventNodeDeregistered, map[string]interface{}{
		"node_id": node.UUID, "status": node.Status,
	})
	return nil
}

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
	s.publishClusterEvent(cluster.EventNodeDraining, map[string]interface{}{
		"node_id": node.UUID, "status": node.Status,
	})
	return nil
}

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
	s.publishClusterEvent(cluster.EventNodeUndrained, map[string]interface{}{
		"node_id": node.UUID, "status": node.Status,
	})
	return nil
}

// ListNodes 返回全部节点记录。
func (s *ClusterService) ListNodes() ([]model.ClusterNode, error) {
	nodes, err := s.nodeRepo.List()
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nodes, nil
}

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
func (s *ClusterService) ScaleServer(serverUUID string, replicas int, preferredNode string) ([]model.ServerAssignment, error) {
	if strings.TrimSpace(serverUUID) == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "server_uuid is required")
	}
	if replicas < 0 {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "replicas must be >= 0")
	}
	if replicas == 0 {
		if err := s.assignRepo.RemoveAll(serverUUID); err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		return []model.ServerAssignment{}, nil
	}

	nodes, err := s.nodeRepo.List()
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	current, err := s.assignRepo.ListByServer(serverUUID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	byUUID := make(map[string]model.ClusterNode, len(nodes))
	for _, node := range nodes {
		byUUID[node.UUID] = node
	}

	activeCurrent := make([]model.ServerAssignment, 0, len(current))
	for _, assignment := range current {
		node, ok := byUUID[assignment.NodeUUID]
		if !ok || !cluster.CanSchedule(node) {
			if err := s.assignRepo.Remove(serverUUID, assignment.NodeUUID); err != nil {
				return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
			}
			continue
		}
		activeCurrent = append(activeCurrent, assignment)
	}
	current = activeCurrent

	if len(current) > replicas {
		sort.SliceStable(current, func(i, j int) bool {
			ni, oki := byUUID[current[i].NodeUUID]
			nj, okj := byUUID[current[j].NodeUUID]
			if !oki {
				return true
			}
			if !okj {
				return false
			}
			return cluster.NodeScore(ni) > cluster.NodeScore(nj)
		})
		for len(current) > replicas {
			removed := current[0]
			if err := s.assignRepo.Remove(serverUUID, removed.NodeUUID); err != nil {
				return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
			}
			current = current[1:]
		}
	}

	if len(current) < replicas {
		selected := cluster.ChooseNodes(nodes, current, replicas-len(current), preferredNode)
		for _, nodeUUID := range selected {
			if err := s.assignRepo.Ensure(serverUUID, nodeUUID); err != nil {
				return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
			}
		}
	}

	assignments, err := s.assignRepo.ListByServer(serverUUID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.syncNodeServerCounts()
	s.publishClusterEvent(cluster.EventServerScaled, map[string]interface{}{
		"server_uuid": serverUUID, "replicas": len(assignments), "assignments": assignments,
	})
	return assignments, nil
}

// EnsureServer 默认保证一个 Server 至少有一个分配；无可用节点时保留为空，等待节点注册。
func (s *ClusterService) EnsureServer(serverUUID string, replicas int, preferredNode string) error {
	_, err := s.ScaleServer(serverUUID, replicas, preferredNode)
	return err
}

// DeleteServer 删除 Server 的全部实例分配。
func (s *ClusterService) DeleteServer(serverUUID string) error {
	if err := s.assignRepo.RemoveAll(serverUUID); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.publishClusterEvent(cluster.EventServerDeleted, map[string]interface{}{
		"server_uuid": serverUUID,
	})
	s.syncNodeServerCounts()
	return nil
}

// ListAssignments 返回 Server 当前实例分配。
func (s *ClusterService) ListAssignments(serverUUID string) ([]model.ServerAssignment, error) {
	assignments, err := s.assignRepo.ListByServer(serverUUID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return assignments, nil
}

// PublishControl 发布 NATS 控制命令，由目标 Worker 执行本地信令操作。
func (s *ClusterService) PublishControl(cmd cluster.ControlCommand) error {
	if err := cmd.Validate(); err != nil {
		return pkg.NewAppError(pkg.INVALID_PARAMS, err.Error())
	}
	s.publishClusterEvent(cluster.EventControlCommand, cmd)
	return nil
}

// ReconcileAll 对账集群状态：回收离线节点并清理不可调度节点上的历史分配。
func (s *ClusterService) ReconcileAll(timeout time.Duration) error {
	if err := s.ReapOffline(timeout); err != nil {
		return err
	}
	nodes, err := s.nodeRepo.List()
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	for _, node := range nodes {
		if cluster.CanSchedule(node) {
			continue
		}
		assignments, listErr := s.assignRepo.ListByNode(node.UUID)
		if listErr != nil {
			continue
		}
		for _, assignment := range assignments {
			_ = s.assignRepo.Remove(assignment.ServerUUID, node.UUID)
		}
	}
	s.syncNodeServerCounts()
	return nil
}

// ResolveServer 返回 Server 当前可路由的 Worker 节点与 workerUrl。
// ClusterStats 是集群控制面健康统计。
type ClusterStats struct {
	TotalNodes    int `json:"total_nodes"`
	ReadyNodes    int `json:"ready_nodes"`
	DrainingNodes int `json:"draining_nodes"`
	OfflineNodes  int `json:"offline_nodes"`
	Assignments   int `json:"assignments"`
}

// Stats 汇总节点与 Server 分配统计。
func (s *ClusterService) Stats() (ClusterStats, error) {
	nodes, err := s.nodeRepo.List()
	if err != nil {
		return ClusterStats{}, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	stats := ClusterStats{TotalNodes: len(nodes)}
	for _, node := range nodes {
		switch node.Status {
		case model.ClusterNodeReady, model.ClusterNodeBusy:
			stats.ReadyNodes++
		case model.ClusterNodeDraining:
			stats.DrainingNodes++
		case model.ClusterNodeOffline:
			stats.OfflineNodes++
		}
		assignments, listErr := s.assignRepo.ListByNode(node.UUID)
		if listErr == nil {
			stats.Assignments += len(assignments)
		}
	}
	return stats, nil
}

func (s *ClusterService) ResolveServer(serverUUID string) (*model.ServerAssignment, *model.ClusterNode, error) {
	assignments, err := s.assignRepo.ListByServer(serverUUID)
	if err != nil {
		return nil, nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	nodes, err := s.nodeRepo.List()
	if err != nil {
		return nil, nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	byUUID := make(map[string]model.ClusterNode, len(nodes))
	for _, node := range nodes {
		byUUID[node.UUID] = node
	}

	var bestAssignment *model.ServerAssignment
	var bestScore float64 = -1
	for i := range assignments {
		if assignments[i].Status == model.ServerAssignmentDraining {
			continue
		}
		node, ok := byUUID[assignments[i].NodeUUID]
		if !ok || !cluster.CanSchedule(node) || node.AdvertiseURL == "" {
			continue
		}
		score := cluster.NodeScore(node)
		if bestAssignment == nil || score < bestScore {
			bestAssignment = &assignments[i]
			bestScore = score
		}
	}
	if bestAssignment == nil {
		return nil, nil, ErrClusterServerUnassigned
	}
	node := byUUID[bestAssignment.NodeUUID]
	return bestAssignment, &node, nil
}

// reconcilePendingServers 在节点心跳后尝试把未分配 Server 调度到该节点。
func (s *ClusterService) reconcilePendingServers(nodeUUID string) {
	if s.serverRepo == nil {
		return
	}
	node, err := s.nodeRepo.GetByUUID(nodeUUID)
	if err != nil || !cluster.CanSchedule(*node) {
		return
	}
	domains, _, err := s.serverRepo.List(1, 10000)
	if err != nil {
		return
	}
	for _, domain := range domains {
		assignments, listErr := s.assignRepo.ListByServer(domain.UUID)
		if listErr != nil {
			continue
		}
		if len(assignments) == 0 {
			_, _ = s.ScaleServer(domain.UUID, 1, nodeUUID)
		}
	}
}

func (s *ClusterService) syncNodeServerCounts() {
	nodes, err := s.nodeRepo.List()
	if err != nil {
		return
	}
	for _, node := range nodes {
		count, err := s.assignRepo.CountByNode(node.UUID)
		if err != nil {
			continue
		}
		if node.ServingServers != int(count) {
			node.ServingServers = int(count)
			_ = s.nodeRepo.Update(&node)
		}
	}
}

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

func (s *ClusterService) publishClusterEvent(event string, payload interface{}) {
	if s.notifier == nil {
		return
	}
	_ = s.notifier.PublishInternal(context.Background(), event, payload)
}
