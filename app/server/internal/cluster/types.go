// Package cluster 提供多节点控制面的领域类型、调度逻辑和 Worker→Agent 客户端。
package cluster

import (
	"encoding/json"
	"strings"
)

// HeartbeatReport 是 Worker/All 节点上报给 Agent 的运行时快照。
type HeartbeatReport struct {
	NodeID               string  `json:"node_id"`
	Status               string  `json:"status,omitempty"`
	AdvertiseURL         string  `json:"advertise_url,omitempty"`
	Rooms                int     `json:"rooms"`
	Connections          int     `json:"connections"`
	ServingServers       int     `json:"serving_servers"`
	LoadPercent          float64 `json:"load_percent"`
	SFUHealthy           *bool   `json:"sfu_healthy,omitempty"`
	DBReplicaLagMs       int64   `json:"db_replica_lag_ms,omitempty"`
	DBReplicaLagDegraded bool    `json:"db_replica_lag_degraded,omitempty"`
}

// RegisterRequest 是 Worker 首次向 Agent 注册时提交的节点信息。
type RegisterRequest struct {
	UUID         string            `json:"uuid"`
	Name         string            `json:"name"`
	Host         string            `json:"host"`
	AdvertiseURL string            `json:"advertise_url"`
	Role         string            `json:"role"`
	SFUProvider  string            `json:"sfu_provider"`
	MaxServers   int               `json:"max_servers"`
	MaxRooms     int               `json:"max_rooms"`
	Labels       map[string]string `json:"labels"`
	NodeSecret   string            `json:"node_secret,omitempty"`
}

// ParseLabels 解析 CLUSTER_LABELS 风格字符串，例如 region=cn,pool=voice。
func ParseLabels(raw string) map[string]string {
	labels := map[string]string{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, ok := strings.Cut(part, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			labels[part] = ""
			continue
		}
		labels[key] = strings.TrimSpace(value)
	}
	return labels
}

// EncodeLabels 序列化标签 map 为 JSON 字符串。
func EncodeLabels(labels map[string]string) string {
	if labels == nil {
		labels = map[string]string{}
	}
	raw, err := json.Marshal(labels)
	if err != nil {
		return "{}"
	}
	return string(raw)
}
