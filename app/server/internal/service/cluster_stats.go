package service

import (
	"GOSpeak/internal/cluster"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"errors"
	"fmt"
	"time"
)

// PublishControl 发布 NATS 控制命令，由目标 Worker 执行本地信令操作。
func (s *ClusterService) PublishControl(cmd cluster.ControlCommand) error {
	if err := cmd.Validate(); err != nil {
		return pkg.NewAppError(pkg.INVALID_PARAMS, err.Error())
	}
	if err := s.publishClusterEvent(cluster.EventControlCommand, cmd); err != nil {
		return pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "publish cluster control command failed")
	}
	return nil
}

// ReconcileAll 对账集群状态：回收离线节点并清理不可调度节点上的历史分配。

// ReconcileAll 对账集群状态：回收离线节点并清理不可调度节点上的历史分配。
func (s *ClusterService) ReconcileAll(timeout time.Duration) error {
	if err := s.ReapOffline(timeout); err != nil {
		return err
	}
	nodes, err := s.nodeRepo.List()
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	var errs []error
	for _, node := range nodes {
		if node.Status != model.ClusterNodeOffline && node.Status != model.ClusterNodeUnhealthy {
			continue
		}
		assignments, listErr := s.assignRepo.ListByNode(node.UUID)
		if listErr != nil {
			errs = append(errs, fmt.Errorf("list assignments for node %s: %w", node.UUID, listErr))
			continue
		}
		for _, assignment := range assignments {
			if err := s.assignRepo.Remove(assignment.ServerUUID, node.UUID); err != nil {
				errs = append(errs, fmt.Errorf("remove assignment %s/%s: %w", assignment.ServerUUID, node.UUID, err))
			}
		}
	}
	s.syncNodeServerCounts()
	if len(errs) > 0 {
		return pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, errors.Join(errs...), "reconcile cluster assignments failed")
	}
	return nil
}

// ResolveServer 返回 Server 当前可路由的 Worker 节点与 workerUrl。
// ClusterStats 是集群控制面健康统计。

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
