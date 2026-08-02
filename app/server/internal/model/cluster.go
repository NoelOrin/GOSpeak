package model

import (
	"encoding/json"
	"time"

	"github.com/nrednav/cuid2"
	"gorm.io/gorm"
)

const (
	ClusterRoleAgent  = "agent"
	ClusterRoleWorker = "worker"
	ClusterRoleAll    = "all"

	ClusterNodePending   = "pending"
	ClusterNodeReady     = "ready"
	ClusterNodeBusy      = "busy"
	ClusterNodeDraining  = "draining"
	ClusterNodeOffline   = "offline"
	ClusterNodeUnhealthy = "unhealthy"

	ServerAssignmentAssigned = "assigned"
	ServerAssignmentDraining = "draining"
)

// ClusterNode 是 Agent 控制面中的节点状态记录。
type ClusterNode struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UUID           string    `gorm:"size:64;uniqueIndex" json:"uuid"`
	Name           string    `gorm:"size:128;uniqueIndex" json:"name"`
	Host           string    `gorm:"size:255" json:"host"`
	AdvertiseURL   string    `gorm:"size:512" json:"advertise_url"`
	Role           string    `gorm:"size:16;index" json:"role"`
	Status         string    `gorm:"size:16;index;default:pending" json:"status"`
	SFUProvider    string    `gorm:"size:64" json:"sfu_provider"`
	MaxServers     int       `gorm:"default:100" json:"max_servers"`
	MaxRooms       int       `gorm:"default:1000" json:"max_rooms"`
	ServingServers int       `gorm:"default:0" json:"serving_servers"`
	Rooms          int       `gorm:"default:0" json:"rooms"`
	Connections    int       `gorm:"default:0" json:"connections"`
	LoadPercent    float64   `gorm:"default:0" json:"load_percent"`
	SFUHealthy     bool      `gorm:"default:true" json:"sfu_healthy"`
	LabelsJSON     string    `gorm:"column:labels_json;type:text" json:"-"`
	LastSeenAt     time.Time `json:"last_seen_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (n *ClusterNode) TableName() string {
	return "cluster_nodes"
}

func (n *ClusterNode) BeforeCreate(_ *gorm.DB) error {
	if n.UUID == "" {
		n.UUID = cuid2.Generate()
	}
	if n.Name == "" {
		n.Name = n.UUID
	}
	if n.Status == "" {
		n.Status = ClusterNodePending
	}
	if n.LastSeenAt.IsZero() {
		n.LastSeenAt = time.Now()
	}
	return nil
}

// LabelMap 返回节点标签；JSON 为空时返回空 map。
func (n *ClusterNode) LabelMap() map[string]string {
	labels := map[string]string{}
	if n == nil || n.LabelsJSON == "" {
		return labels
	}
	if err := json.Unmarshal([]byte(n.LabelsJSON), &labels); err != nil {
		return map[string]string{}
	}
	if labels == nil {
		labels = map[string]string{}
	}
	return labels
}

// SetLabels 序列化节点标签。
func (n *ClusterNode) SetLabels(labels map[string]string) {
	if labels == nil {
		labels = map[string]string{}
	}
	raw, err := json.Marshal(labels)
	if err != nil {
		raw = []byte("{}")
	}
	n.LabelsJSON = string(raw)
}

// ServerAssignment 表示一个 Server（Domain）在某个节点上的实例分配。
type ServerAssignment struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ServerUUID string    `gorm:"size:64;uniqueIndex:idx_server_assignment_server_node,priority:1;index;not null" json:"server_uuid"`
	NodeUUID   string    `gorm:"size:64;uniqueIndex:idx_server_assignment_server_node,priority:2;index;not null" json:"node_uuid"`
	Status     string    `gorm:"size:16;default:assigned" json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (ServerAssignment) TableName() string {
	return "server_assignments"
}

func (a *ServerAssignment) BeforeCreate(_ *gorm.DB) error {
	if a.Status == "" {
		a.Status = ServerAssignmentAssigned
	}
	return nil
}
