package cluster

import (
	"sort"

	"GOSpeak/internal/model"
)

// NodeRequirement 表达 Server 对节点的调度约束。
type NodeRequirement struct {
	SFUProvider string
	Labels      map[string]string
}

// Matches 判断节点是否满足 Provider 与标签约束。
func (r NodeRequirement) Matches(node model.ClusterNode) bool {
	if r.SFUProvider != "" && node.SFUProvider != r.SFUProvider {
		return false
	}
	if len(r.Labels) == 0 {
		return true
	}
	labels := node.LabelMap()
	for key, value := range r.Labels {
		if labels[key] != value {
			return false
		}
	}
	return true
}

// CanSchedule 判断节点当前是否接受新的 Server 分配。
func CanSchedule(node model.ClusterNode) bool {
	switch node.Status {
	case model.ClusterNodeReady, model.ClusterNodeBusy:
	default:
		return false
	}
	return node.SFUHealthy
}

// NodeScore 返回越小越优先的节点负载评分。
// 当前按实例数、房间数和上报负载的加权值近似计算。
func NodeScore(node model.ClusterNode) float64 {
	servers := 0.0
	if node.MaxServers > 0 {
		servers = float64(node.ServingServers) / float64(node.MaxServers)
	}
	rooms := 0.0
	if node.MaxRooms > 0 {
		rooms = float64(node.Rooms) / float64(node.MaxRooms)
	}
	load := node.LoadPercent / 100
	if load < 0 {
		load = 0
	}
	if load > 1 {
		load = 1
	}
	return servers*0.4 + rooms*0.3 + load*0.3
}

// ChooseNodes 从可调度节点中选择 count 个未分配给指定 Server 的节点。
// 优先选择 preferred 中出现的节点，再按负载评分升序选择。
func ChooseNodes(nodes []model.ClusterNode, current []model.ServerAssignment, count int, preferred ...string) []string {
	return ChooseNodesWithRequirement(nodes, current, count, preferred, NodeRequirement{})
}

// ChooseNodesWithRequirement 从满足标签/Provider 约束的可调度节点中选择 count 个。
func ChooseNodesWithRequirement(nodes []model.ClusterNode, current []model.ServerAssignment, count int, preferred []string, requirement NodeRequirement) []string {
	if count <= 0 {
		return nil
	}
	currentSet := make(map[string]struct{}, len(current))
	for _, assignment := range current {
		currentSet[assignment.NodeUUID] = struct{}{}
	}
	preferredSet := make(map[string]struct{}, len(preferred))
	for _, nodeUUID := range preferred {
		preferredSet[nodeUUID] = struct{}{}
	}

	candidates := make([]model.ClusterNode, 0, len(nodes))
	for _, node := range nodes {
		if !CanSchedule(node) || !requirement.Matches(node) {
			continue
		}
		if _, ok := currentSet[node.UUID]; ok {
			continue
		}
		candidates = append(candidates, node)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		_, pi := preferredSet[candidates[i].UUID]
		_, pj := preferredSet[candidates[j].UUID]
		if pi != pj {
			return pi
		}
		return NodeScore(candidates[i]) < NodeScore(candidates[j])
	})

	out := make([]string, 0, count)
	for i := 0; i < len(candidates) && len(out) < count; i++ {
		out = append(out, candidates[i].UUID)
	}
	return out
}
