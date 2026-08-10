package handler

import (
	"strings"
	"time"

	"GOSpeak/internal/cluster"
	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

// ClusterHandler 提供集群控制面 HTTP API。
type ClusterHandler struct {
	clusterSvc *service.ClusterService
	cfg        *config.Config
}

func NewClusterHandler(clusterSvc *service.ClusterService, cfg *config.Config) *ClusterHandler {
	return &ClusterHandler{clusterSvc: clusterSvc}
}

type RegisterNodeRequest struct {
	UUID         string            `json:"uuid" binding:"required"`
	Name         string            `json:"name"`
	Host         string            `json:"host"`
	AdvertiseURL string            `json:"advertise_url"`
	Role         string            `json:"role"`
	Status       string            `json:"status"`
	SFUProvider  string            `json:"sfu_provider"`
	MaxServers   int               `json:"max_servers"`
	MaxRooms     int               `json:"max_rooms"`
	Labels       map[string]string `json:"labels"`
	NodeSecret   string            `json:"node_secret"`
}

// Register 注册或更新节点。
func (h *ClusterHandler) Register(c *gin.Context) {
	var req RegisterNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	saved, err := h.clusterSvc.RegisterNodeParams(
		req.UUID, req.Name, req.Host, req.AdvertiseURL,
		req.Role, req.Status, req.SFUProvider,
		req.MaxServers, req.MaxRooms, req.Labels, req.NodeSecret,
	)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"node": saved, "labels": saved.LabelMap()})
}

type HeartbeatRequest struct {
	NodeID         string  `json:"node_id" binding:"required"`
	Status         string  `json:"status"`
	AdvertiseURL   string  `json:"advertise_url"`
	Rooms          int     `json:"rooms"`
	Connections    int     `json:"connections"`
	ServingServers int     `json:"serving_servers"`
	LoadPercent    float64 `json:"load_percent"`
	SFUHealthy     *bool   `json:"sfu_healthy"`
}

// Heartbeat 接收节点心跳。
func (h *ClusterHandler) Heartbeat(c *gin.Context) {
	var req HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	node, err := h.clusterSvc.Heartbeat(req.NodeID, cluster.HeartbeatReport{
		NodeID:         req.NodeID,
		Status:         req.Status,
		AdvertiseURL:   req.AdvertiseURL,
		Rooms:          req.Rooms,
		Connections:    req.Connections,
		ServingServers: req.ServingServers,
		LoadPercent:    req.LoadPercent,
		SFUHealthy:     req.SFUHealthy,
	})
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, node)
}

type DeregisterNodeRequest struct {
	NodeID string `json:"node_id" binding:"required"`
}

// Deregister 注销节点。
func (h *ClusterHandler) Deregister(c *gin.Context) {
	var req DeregisterNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.clusterSvc.DeregisterNode(req.NodeID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

// Drain 标记节点 draining，停止新分配。
func (h *ClusterHandler) Drain(c *gin.Context) {
	var req DeregisterNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.clusterSvc.DrainNode(req.NodeID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

// Undrain 恢复节点 ready，允许继续调度。
func (h *ClusterHandler) Undrain(c *gin.Context) {
	var req DeregisterNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	timeout := time.Duration(0)
	if h.cfg != nil {
		timeout = h.cfg.ClusterHeartbeatTimeoutDuration()
	}
	if err := h.clusterSvc.UndrainNode(req.NodeID, timeout); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

// List 返回节点列表。
func (h *ClusterHandler) List(c *gin.Context) {
	nodes, err := h.clusterSvc.ListNodes()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	views := make([]gin.H, 0, len(nodes))
	for _, node := range nodes {
		views = append(views, gin.H{"node": node, "labels": node.LabelMap()})
	}
	pkg.Success(c, gin.H{"nodes": views})
}

// Stats 返回集群健康统计。
func (h *ClusterHandler) Stats(c *gin.Context) {
	stats, err := h.clusterSvc.Stats()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, stats)
}

type ScaleServerRequest struct {
	ServerUUID  string            `json:"server_uuid" binding:"required"`
	Replicas    int               `json:"replicas" binding:"required"`
	SFUProvider string            `json:"sfu_provider"`
	Labels      map[string]string `json:"labels"`
}

// Scale 调整 Server 实例组副本数。
func (h *ClusterHandler) Scale(c *gin.Context) {
	var req ScaleServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	assignments, err := h.clusterSvc.ScaleServerWithRequirement(req.ServerUUID, req.Replicas, "", cluster.NodeRequirement{
		SFUProvider: req.SFUProvider,
		Labels:      req.Labels,
	})
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"assignments": assignments})
}

type ServerUUIDRequest struct {
	ServerUUID string `json:"server_uuid" binding:"required"`
}

type AutoScaleServerRequest struct {
	ServerUUID string `json:"server_uuid" binding:"required"`
	Replicas   int    `json:"replicas" binding:"required"`
}

// ListAssignments 返回 Server 当前实例分配。
func (h *ClusterHandler) ListAssignments(c *gin.Context) {
	var req ServerUUIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	assignments, err := h.clusterSvc.ListAssignments(req.ServerUUID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"assignments": assignments})
}

// DrainServer 将 Server 全部副本标记为 draining，配合灰度下线。
func (h *ClusterHandler) DrainServer(c *gin.Context) {
	var req ServerUUIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.clusterSvc.MarkServerAssignmentsDraining(req.ServerUUID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

// AutoScaleServer 按目标副本数扩缩 Server；无可用节点时不动作。
func (h *ClusterHandler) AutoScaleServer(c *gin.Context) {
	var req AutoScaleServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.clusterSvc.AutoScale(req.ServerUUID, req.Replicas); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

type ResolveServerRequest struct {
	ServerUUID string `json:"server_uuid" binding:"required"`
}

// Resolve 返回 Server 当前可路由节点。
func (h *ClusterHandler) Resolve(c *gin.Context) {
	var req ResolveServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	assignment, node, err := h.clusterSvc.ResolveServer(req.ServerUUID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	workerURL := strings.TrimSpace(node.AdvertiseURL)
	pkg.Success(c, gin.H{
		"assignment": assignment,
		"node":       node,
		"worker_url": workerURL,
	})
}
