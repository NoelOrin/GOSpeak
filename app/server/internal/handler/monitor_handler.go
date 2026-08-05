package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"

	"GOSpeak/internal/bus"
	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/redis"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/service"
	gpsignal "GOSpeak/internal/signal"
)

// MonitorHandler 服务监控 SSE 处理器
type MonitorHandler struct {
	startTime  time.Time
	signalHub  *gpsignal.Hub
	eventBus   bus.EventBus
	dbPath     string
	cpuSampler *cpuSampler
	clusterSvc *service.ClusterService
}

func NewMonitorHandler(signalHub *gpsignal.Hub, cfg *config.Config, eventBus bus.EventBus, clusterSvc *service.ClusterService) *MonitorHandler {
	h := &MonitorHandler{
		startTime:  time.Now(),
		signalHub:  signalHub,
		eventBus:   eventBus,
		dbPath:     cfg.DBPath,
		clusterSvc: clusterSvc,
	}
	h.cpuSampler = newCPUSampler()
	return h
}

// HealthStream SSE 流式推送服务健康指标
// 鉴权由 protected 路由组统一完成（Header/cookie 均可）；此处仅做防御性 admin 校验，
// 避免误注册到公开路由时泄露监控指标。
func (h *MonitorHandler) HealthStream(c *gin.Context) {
	roleVal, ok := c.Get("role")
	if !ok {
		pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
		return
	}
	role, _ := roleVal.(string)
	if role != "admin" {
		pkg.Fail(c, pkg.FORBIDDEN)
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()

	// 连接建立立即推送一次，避免前端空窗
	snap0 := h.collect()
	data0, _ := json.Marshal(snap0)
	_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", data0)
	c.Writer.Flush()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	clientGone := c.Request.Context().Done()

	for {
		select {
		case <-clientGone:
			return
		case <-ticker.C:
			snap := h.collect()
			data, _ := json.Marshal(snap)
			if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", data); err != nil {
				return
			}
			c.Writer.Flush()
		}
	}
}

type healthSnapshot struct {
	Timestamp string `json:"timestamp"`
	Uptime    string `json:"uptime"`

	// Go runtime
	NumGoroutine  int     `json:"num_goroutine"`
	NumCPU        int     `json:"num_cpu"`
	AllocMB       float64 `json:"alloc_mb"`
	TotalAllocMB  float64 `json:"total_alloc_mb"`
	SysMB         float64 `json:"sys_mb"`
	NumGC         uint32  `json:"num_gc"`
	HeapObjects   uint64  `json:"heap_objects"`
	GCPauseMs     float64 `json:"gc_pause_ms"`
	GCCPUFraction float64 `json:"gc_cpu_fraction"`

	// 进程/系统
	PID         int     `json:"pid"`
	CPUPercent  float64 `json:"cpu_percent"`
	DiskUsedMB  float64 `json:"disk_used_mb"`
	DiskTotalMB float64 `json:"disk_total_mb"`
	DiskPercent float64 `json:"disk_percent"`

	// 业务
	HubRoomCount        int    `json:"hub_room_count"`
	HubParticipantCount int    `json:"hub_participant_count"`
	HubOnlineUserCount  int    `json:"hub_online_user_count"`
	WSClientDropped     uint64 `json:"ws_client_dropped"`

	// DB 连接池
	DBConnected      bool  `json:"db_connected"`
	DBInUse          int   `json:"db_in_use"`
	DBIdle           int   `json:"db_idle"`
	DBMaxOpen        int   `json:"db_max_open"`
	DBWaitCount      int64 `json:"db_wait_count"`
	DBWaitDurationMs int64 `json:"db_wait_duration_ms"`

	// Redis
	RedisConnected        bool    `json:"redis_connected"`
	RedisPingMs           int64   `json:"redis_ping_ms"`
	RedisDBSize           int64   `json:"redis_db_size"`
	RedisUsedMemoryMB     float64 `json:"redis_used_memory_mb"`
	RedisUsedMemoryPeakMB float64 `json:"redis_used_memory_peak_mb"`
	RedisConnectedClients int64   `json:"redis_connected_clients"`

	// EventBus
	EventBusMode                 string `json:"eventbus_mode"`
	EventBusConnected            bool   `json:"eventbus_connected"`
	EventBusInstanceID           string `json:"eventbus_instance_id"`
	EventBusFallbackFromExternal bool   `json:"eventbus_fallback_from_external"`
	EventBusDroppedPublish       uint64 `json:"eventbus_dropped_publish"`

	// Shared multi-instance backends
	AuthStoreBackend string `json:"auth_store_backend"`

	// Cluster
	ClusterTotalNodes  int `json:"cluster_total_nodes"`
	ClusterReadyNodes  int `json:"cluster_ready_nodes"`
	ClusterAssignments int `json:"cluster_assignments"`
}

func (h *MonitorHandler) collect() healthSnapshot {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	now := time.Now()
	snap := healthSnapshot{
		Timestamp:     now.Format("15:04:05"),
		Uptime:        time.Since(h.startTime).Truncate(time.Second).String(),
		NumGoroutine:  runtime.NumGoroutine(),
		NumCPU:        runtime.NumCPU(),
		AllocMB:       roundMB(m.Alloc),
		TotalAllocMB:  roundMB(m.TotalAlloc),
		SysMB:         roundMB(m.Sys),
		NumGC:         m.NumGC,
		HeapObjects:   m.HeapObjects,
		GCPauseMs:     float64(m.PauseTotalNs) / 1e6,
		GCCPUFraction: m.GCCPUFraction,
		PID:           os.Getpid(),
		CPUPercent:    h.cpuSampler.percent(),
	}

	// 磁盘：取 DB 文件所在分区使用率
	if path := h.dbPath; path != "" {
		if stat, err := disk.Usage(filepath2Dir(path)); err == nil {
			snap.DiskUsedMB = roundMB(uint64(stat.Used))
			snap.DiskTotalMB = roundMB(uint64(stat.Total))
			snap.DiskPercent = round2(stat.UsedPercent)
		}
	}

	// 信令面业务统计
	if h.signalHub != nil {
		hs := h.signalHub.GetStats()
		snap.HubRoomCount = hs.RoomCount
		snap.HubParticipantCount = hs.ParticipantCount
		snap.HubOnlineUserCount = hs.OnlineUserCount
		snap.WSClientDropped = hs.WSClientDropped
	}

	// DB 连接池
	st := repository.DBStats()
	snap.DBInUse = st.InUse
	snap.DBIdle = st.Idle
	snap.DBMaxOpen = st.MaxOpenConnections
	snap.DBWaitCount = st.WaitCount
	snap.DBWaitDurationMs = st.WaitDuration.Milliseconds()
	if err := repository.DBPing(); err == nil {
		snap.DBConnected = true
	}

	// Redis 详细状态
	rs := redis.GetStats()
	snap.RedisConnected = rs.Connected
	snap.RedisPingMs = rs.PingMs
	snap.RedisDBSize = rs.DBSize
	snap.RedisUsedMemoryMB = rs.UsedMemoryMB
	snap.RedisUsedMemoryPeakMB = rs.UsedMemoryPeakMB
	snap.RedisConnectedClients = rs.ConnectedClients

	es := bus.GetStats(h.eventBus)
	snap.EventBusMode = es.Mode
	snap.EventBusConnected = es.Connected
	snap.EventBusInstanceID = es.InstanceID
	snap.EventBusFallbackFromExternal = es.FallbackFromExternal
	snap.EventBusDroppedPublish = es.DroppedPublish

	if redis.IsConnected() {
		snap.AuthStoreBackend = "redis"
	} else if name := redis.AuthBackendName(); name != "" {
		snap.AuthStoreBackend = name
	} else {
		snap.AuthStoreBackend = "none"
	}

	if h.clusterSvc != nil {
		stats, err := h.clusterSvc.Stats()
		if err == nil {
			snap.ClusterTotalNodes = stats.TotalNodes
			snap.ClusterReadyNodes = stats.ReadyNodes
			snap.ClusterAssignments = stats.Assignments
		}
	}

	return snap
}

func roundMB(bytes uint64) float64 {
	return float64(int(float64(bytes)/1024/1024*100)) / 100
}

func round2(v float64) float64 {
	return float64(int(v*100)) / 100
}

// filepath2Dir 取路径所在目录，路径不存在时回退到当前目录。
func filepath2Dir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == os.PathSeparator {
			return path[:i]
		}
	}
	return "."
}

// cpuSampler 后台定期采样整机 CPU 使用率，collect 只读缓存值，避免阻塞 SSE。
type cpuSampler struct {
	mu   sync.RWMutex
	pct  float64
	stop chan struct{}
}

func newCPUSampler() *cpuSampler {
	s := &cpuSampler{stop: make(chan struct{})}
	go s.loop()
	return s
}

func (s *cpuSampler) loop() {
	// 首次采样填充初值
	if pcts, err := cpu.Percent(time.Second, false); err == nil && len(pcts) > 0 {
		s.mu.Lock()
		s.pct = round2(pcts[0])
		s.mu.Unlock()
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			if pcts, err := cpu.Percent(time.Second, false); err == nil && len(pcts) > 0 {
				s.mu.Lock()
				s.pct = round2(pcts[0])
				s.mu.Unlock()
			}
		}
	}
}

func (s *cpuSampler) percent() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pct
}
