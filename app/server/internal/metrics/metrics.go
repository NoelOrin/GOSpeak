// Package metrics 暴露 Prometheus 指标，供可观测栈采集。
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Snapshot 是应用侧业务指标快照，由 monitor 采集后转换为 Prometheus 指标。
type Snapshot struct {
	CPUPercent float64

	HubRoomCount        int
	HubParticipantCount int
	HubOnlineUserCount  int
	WSClientDropped     uint64

	DBConnected             bool
	DBInUse                 int
	DBIdle                  int
	DBMaxOpen               int
	DBWaitCount             int64
	DBWaitDurationMs        int64
	DBReplicaLagMs          int64
	DBReplicaLagThresholdMs int64
	DBReplicaLagDegraded    bool

	EventBusConnected      bool
	EventBusDroppedPublish uint64

	ClusterTotalNodes    int
	ClusterReadyNodes    int
	ClusterDrainingNodes int
	ClusterOfflineNodes  int
	ClusterAssignments   int

	DiskUsedMB  float64
	DiskTotalMB float64
	DiskPercent float64
}

// Server 持有 Prometheus registry 与 HTTP 请求指标。
type Server struct {
	registry        *prometheus.Registry
	snapshot        func() Snapshot
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec

	up                     prometheus.Gauge
	wsRooms                prometheus.Gauge
	wsParticipants         prometheus.Gauge
	wsOnlineUsers          prometheus.Gauge
	wsClientDropped        prometheus.Gauge
	dbConnected            prometheus.Gauge
	dbConnectionsInUse     prometheus.Gauge
	dbConnectionsIdle      prometheus.Gauge
	dbConnectionsMax       prometheus.Gauge
	dbWaitCount            prometheus.Gauge
	dbWaitDurationSeconds  prometheus.Gauge
	dbReplicaLagSeconds    prometheus.Gauge
	dbReplicaLagDegraded   prometheus.Gauge
	eventbusConnected      prometheus.Gauge
	eventbusDroppedPublish prometheus.Gauge
	clusterNodes           prometheus.Gauge
	clusterReadyNodes      prometheus.Gauge
	clusterDrainingNodes   prometheus.Gauge
	clusterOfflineNodes    prometheus.Gauge
	clusterAssignments     prometheus.Gauge
	diskUsedBytes          prometheus.Gauge
	diskTotalBytes         prometheus.Gauge
	diskPercent            prometheus.Gauge
	cpuPercent             prometheus.Gauge
}

// New 创建指标服务。snapshot 为空时业务 gauge 保持 0。
func New(snapshot func() Snapshot) *Server {
	s := &Server{
		registry: prometheus.NewRegistry(),
		snapshot: snapshot,
		requestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gospeak_http_requests_total",
			Help: "Total HTTP requests handled by GOSpeak.",
		}, []string{"method", "path", "status"}),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "gospeak_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_up",
			Help: "1 when this process is serving metrics.",
		}),
		wsRooms: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_ws_rooms",
			Help: "Current local WebSocket room count.",
		}),
		wsParticipants: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_ws_participants",
			Help: "Current local WebSocket participants in rooms.",
		}),
		wsOnlineUsers: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_ws_online_users",
			Help: "Current local online WebSocket users.",
		}),
		wsClientDropped: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_ws_client_dropped",
			Help: "Dropped WebSocket clients count observed by this process.",
		}),
		dbConnected: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_db_connected",
			Help: "1 when the database is reachable.",
		}),
		dbConnectionsInUse: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_db_connections_in_use",
			Help: "Database connections currently in use.",
		}),
		dbConnectionsIdle: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_db_connections_idle",
			Help: "Idle database connections.",
		}),
		dbConnectionsMax: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_db_connections_max",
			Help: "Maximum configured database connections.",
		}),
		dbWaitCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_db_wait_count",
			Help: "Database connection wait count observed by this process.",
		}),
		dbWaitDurationSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_db_wait_duration_seconds",
			Help: "Database connection wait duration in seconds observed by this process.",
		}),
		dbReplicaLagSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_db_replica_lag_seconds",
			Help: "Read replica replication lag in seconds observed by this process.",
		}),
		dbReplicaLagDegraded: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_db_replica_degraded",
			Help: "1 when read replica lag exceeds the configured threshold.",
		}),
		eventbusConnected: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_eventbus_connected",
			Help: "1 when the NATS event bus is connected.",
		}),
		eventbusDroppedPublish: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_eventbus_dropped_publish",
			Help: "Dropped event bus publishes observed by this process.",
		}),
		clusterNodes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_cluster_nodes",
			Help: "Total cluster nodes known to the agent.",
		}),
		clusterReadyNodes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_cluster_ready_nodes",
			Help: "Cluster nodes currently ready or busy.",
		}),
		clusterDrainingNodes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_cluster_draining_nodes",
			Help: "Cluster nodes in draining state.",
		}),
		clusterOfflineNodes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_cluster_offline_nodes",
			Help: "Cluster nodes in offline state.",
		}),
		clusterAssignments: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_cluster_assignments",
			Help: "Current server assignment count.",
		}),
		diskUsedBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_disk_used_bytes",
			Help: "Disk used bytes on the database partition.",
		}),
		diskTotalBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_disk_total_bytes",
			Help: "Disk total bytes on the database partition.",
		}),
		diskPercent: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_disk_percent",
			Help: "Disk usage percentage on the database partition.",
		}),
		cpuPercent: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gospeak_cpu_percent",
			Help: "Sampled host CPU usage percentage.",
		}),
	}

	s.registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		s.requestsTotal,
		s.requestDuration,
		s.up,
		s.wsRooms,
		s.wsParticipants,
		s.wsOnlineUsers,
		s.wsClientDropped,
		s.dbConnected,
		s.dbConnectionsInUse,
		s.dbConnectionsIdle,
		s.dbConnectionsMax,
		s.dbWaitCount,
		s.dbWaitDurationSeconds,
		s.dbReplicaLagSeconds,
		s.dbReplicaLagDegraded,
		s.eventbusConnected,
		s.eventbusDroppedPublish,
		s.clusterNodes,
		s.clusterReadyNodes,
		s.clusterDrainingNodes,
		s.clusterOfflineNodes,
		s.clusterAssignments,
		s.diskUsedBytes,
		s.diskTotalBytes,
		s.diskPercent,
		s.cpuPercent,
	)

	return s
}

// Middleware 记录 Gin HTTP 请求计数与耗时。
func (s *Server) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		status := strconv.Itoa(c.Writer.Status())
		s.requestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		s.requestDuration.WithLabelValues(c.Request.Method, path).Observe(time.Since(start).Seconds())
	}
}

// Handler 返回 /metrics HTTP 处理器，抓取前刷新业务 gauge。
func (s *Server) Handler() http.Handler {
	inner := promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.refresh()
		inner.ServeHTTP(w, r)
	})
}

// RequireToken 为 /metrics 增加可选的 Bearer token 鉴权；token 为空时直接放行。
func RequireToken(next http.Handler, token string) http.Handler {
	if token == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) refresh() {
	s.up.Set(1)
	if s.snapshot == nil {
		return
	}
	snap := s.snapshot()

	s.cpuPercent.Set(snap.CPUPercent)
	s.wsRooms.Set(float64(snap.HubRoomCount))
	s.wsParticipants.Set(float64(snap.HubParticipantCount))
	s.wsOnlineUsers.Set(float64(snap.HubOnlineUserCount))
	s.wsClientDropped.Set(float64(snap.WSClientDropped))

	if snap.DBConnected {
		s.dbConnected.Set(1)
	} else {
		s.dbConnected.Set(0)
	}
	s.dbConnectionsInUse.Set(float64(snap.DBInUse))
	s.dbConnectionsIdle.Set(float64(snap.DBIdle))
	s.dbConnectionsMax.Set(float64(snap.DBMaxOpen))
	s.dbWaitCount.Set(float64(snap.DBWaitCount))
	s.dbWaitDurationSeconds.Set(float64(snap.DBWaitDurationMs) / 1000)
	s.dbReplicaLagSeconds.Set(float64(snap.DBReplicaLagMs) / 1000)
	if snap.DBReplicaLagDegraded {
		s.dbReplicaLagDegraded.Set(1)
	} else {
		s.dbReplicaLagDegraded.Set(0)
	}

	if snap.EventBusConnected {
		s.eventbusConnected.Set(1)
	} else {
		s.eventbusConnected.Set(0)
	}
	s.eventbusDroppedPublish.Set(float64(snap.EventBusDroppedPublish))

	s.clusterNodes.Set(float64(snap.ClusterTotalNodes))
	s.clusterReadyNodes.Set(float64(snap.ClusterReadyNodes))
	s.clusterDrainingNodes.Set(float64(snap.ClusterDrainingNodes))
	s.clusterOfflineNodes.Set(float64(snap.ClusterOfflineNodes))
	s.clusterAssignments.Set(float64(snap.ClusterAssignments))

	s.diskUsedBytes.Set(snap.DiskUsedMB * 1024 * 1024)
	s.diskTotalBytes.Set(snap.DiskTotalMB * 1024 * 1024)
	s.diskPercent.Set(snap.DiskPercent)
}
