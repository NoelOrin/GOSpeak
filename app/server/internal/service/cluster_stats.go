package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"GOSpeak/internal/bus"
	"GOSpeak/internal/cluster"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
)

// PublishControl 发布持久化控制命令，由目标 Worker 消费执行。
// JetStream 保证 Worker 离线期间不丢失，失败由消费者 Nak 重试。
func (s *ClusterService) PublishControl(cmd cluster.ControlCommand) error {
	if err := cmd.Validate(); err != nil {
		return pkg.NewAppError(pkg.INVALID_PARAMS, err.Error())
	}
	if s.controlQueue != nil {
		payload, err := json.Marshal(cmd)
		if err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		if err := s.controlQueue.Publish(context.Background(), bus.JobEnvelope{
			ID:      fmt.Sprintf("control-%d", time.Now().UnixNano()),
			Type:    "cluster.control",
			Payload: payload,
		}); err != nil {
			return pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "publish cluster control command failed")
		}
		return nil
	}
	// 兼容未注入队列的旧路径。
	if err := s.publishClusterEvent(cluster.EventControlCommand, cmd); err != nil {
		return pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "publish cluster control command failed")
	}
	return nil
}

// ReconcileAll 清理 offline/unhealthy 节点上的历史分配。
// 不负责心跳超时判定，由调用方定时 ReapOffline，避免 Agent 重启误杀在线 Worker。

// ReconcileAll 清理 offline/unhealthy 节点上的历史分配。
// 不负责心跳超时判定，由调用方定时 ReapOffline，避免 Agent 重启误杀在线 Worker。
func (s *ClusterService) ReconcileAll(timeout time.Duration) error {
	_ = timeout
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
