package service

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"GOSpeak/internal/cluster"
	"GOSpeak/internal/config"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"gorm.io/gorm"
)

var (
	ErrClusterNodeNotFound    = pkg.NewAppError(pkg.NOT_FOUND, "cluster node not found")
	ErrClusterServerUnassigned = pkg.NewAppError(pkg.NOT_FOUND, "no healthy worker assigned to server")
)

// ClusterService 是 Agent 控制面的节点与 Server 分配服务。
type ClusterService struct {
	nodeRepo  *repository.ClusterNodeRepository
	assignRepo *repository.ServerAssignmentRepository
}

func NewClusterService(
	nodeRepo *repository.ClusterNodeRepository,
	assignRepo *repository.ServerAssignmentRepository,
) *ClusterService {
	return &ClusterService{nodeRepo: nodeRepo, assignRepo: assignRepo}
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
		AdvertiseURL: cfg.ClusterAdvertiseURL,
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
		existing.AdvertiseURL = cfg.ClusterAdvertiseURL
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

	if len(current) > replicas {
		byUUID := make(map[string]model.ClusterNode, len(nodes))
		for _, node := range nodes {
			byUUID[node.UUID] = node
		}
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

// ResolveServer 返回 Server 当前可路由的 Worker 节点与 workerUrl。
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
