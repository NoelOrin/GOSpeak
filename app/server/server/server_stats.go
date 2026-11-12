package server

import (
	"database/sql"

	"GOSpeak/internal/authstate"
	"GOSpeak/internal/bus"
	"GOSpeak/internal/handler"
	"GOSpeak/internal/metrics"
	"GOSpeak/internal/repository"
)

// serverInfraStats 将 repository/bus 的统计能力适配为 handler.InfraStats，
// 使 HTTP handler 不直接依赖基础设施包。
type serverInfraStats struct {
	eventBus bus.EventBus
}

func newServerInfraStats(eventBus bus.EventBus) handler.InfraStats {
	return serverInfraStats{eventBus: eventBus}
}

func (s serverInfraStats) DBStats() sql.DBStats { return repository.DBStats() }
func (s serverInfraStats) DBPing() error        { return repository.DBPing() }

func (s serverInfraStats) AuthStoreBackend() string {
	if name := authstate.BackendName(); name != "" {
		return name
	}
	return "none"
}
func (s serverInfraStats) BusMode() string {
	return bus.GetStats(s.eventBus).Mode
}
func (s serverInfraStats) BusConnected() bool {
	return bus.GetStats(s.eventBus).Connected
}
func (s serverInfraStats) BusInstanceID() string {
	return bus.GetStats(s.eventBus).InstanceID
}
func (s serverInfraStats) BusDroppedPublish() uint64 {
	return bus.GetStats(s.eventBus).DroppedPublish
}

// toMetricsSnapshot 将健康快照转换为 Prometheus 指标，适配逻辑留在组合根。
func toMetricsSnapshot(snap handler.HealthSnapshot) metrics.Snapshot {
	return metrics.Snapshot{
		CPUPercent:             snap.CPUPercent,
		HubRoomCount:           snap.HubRoomCount,
		HubParticipantCount:    snap.HubParticipantCount,
		HubOnlineUserCount:     snap.HubOnlineUserCount,
		WSClientDropped:        snap.WSClientDropped,
		DBConnected:            snap.DBConnected,
		DBInUse:                snap.DBInUse,
		DBIdle:                 snap.DBIdle,
		DBMaxOpen:              snap.DBMaxOpen,
		DBWaitCount:            snap.DBWaitCount,
		DBWaitDurationMs:       snap.DBWaitDurationMs,
		EventBusConnected:      snap.EventBusConnected,
		EventBusDroppedPublish: snap.EventBusDroppedPublish,
		DiskUsedMB:             snap.DiskUsedMB,
		DiskTotalMB:            snap.DiskTotalMB,
		DiskPercent:            snap.DiskPercent,
	}
}
