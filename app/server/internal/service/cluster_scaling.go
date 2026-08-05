package service

import (
	"GOSpeak/internal/cluster"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
)

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
	if err := s.publishClusterEvent(cluster.EventServerScaled, map[string]interface{}{
		"server_uuid": serverUUID, "replicas": len(assignments), "assignments": assignments,
	}); err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "publish server scaled event failed")
	}
	return assignments, nil
}

// EnsureServer 默认保证一个 Server 至少有一个分配；无可用节点时保留为空，等待节点注册。

// EnsureServer 默认保证一个 Server 至少有一个分配；无可用节点时保留为空，等待节点注册。
func (s *ClusterService) EnsureServer(serverUUID string, replicas int, preferredNode string) error {
	_, err := s.ScaleServer(serverUUID, replicas, preferredNode)
	return err
}

// DeleteServer 删除 Server 的全部实例分配。

// DeleteServer 删除 Server 的全部实例分配。
func (s *ClusterService) DeleteServer(serverUUID string) error {
	if err := s.assignRepo.RemoveAll(serverUUID); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := s.publishClusterEvent(cluster.EventServerDeleted, map[string]interface{}{
		"server_uuid": serverUUID,
	}); err != nil {
		return pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "publish server deleted event failed")
	}
	s.syncNodeServerCounts()
	return nil
}

// ListAssignments 返回 Server 当前实例分配。
// AutoScale 按目标副本数扩缩 Server；无可用节点时不动作。

// ListAssignments 返回 Server 当前实例分配。
// AutoScale 按目标副本数扩缩 Server；无可用节点时不动作。
func (s *ClusterService) AutoScale(serverUUID string, targetReplicas int) error {
	stats, err := s.Stats()
	if err != nil {
		return err
	}
	if stats.ReadyNodes == 0 {
		return nil
	}
	_, err = s.ScaleServer(serverUUID, targetReplicas, "")
	return err
}

// MarkServerAssignmentsDraining 标记 Server 全部副本为 draining，配合灰度下线。

// MarkServerAssignmentsDraining 标记 Server 全部副本为 draining，配合灰度下线。
func (s *ClusterService) MarkServerAssignmentsDraining(serverUUID string) error {
	assignments, err := s.assignRepo.ListByServer(serverUUID)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	var errs []error
	for _, assignment := range assignments {
		if err := s.assignRepo.UpdateStatus(serverUUID, assignment.NodeUUID, model.ServerAssignmentDraining); err != nil {
			errs = append(errs, fmt.Errorf("mark assignment %s/%s draining: %w", serverUUID, assignment.NodeUUID, err))
		}
	}
	if len(errs) > 0 {
		return pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, errors.Join(errs...), "mark server assignments draining failed")
	}
	return nil
}

func (s *ClusterService) ListAssignments(serverUUID string) ([]model.ServerAssignment, error) {
	assignments, err := s.assignRepo.ListByServer(serverUUID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return assignments, nil
}

// PublishControl 发布 NATS 控制命令，由目标 Worker 执行本地信令操作。

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
			if _, err := s.ScaleServer(domain.UUID, 1, nodeUUID); err != nil {
				log.Printf("[cluster] reconcile scale failed server=%s: %v", domain.UUID, err)
			}
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
