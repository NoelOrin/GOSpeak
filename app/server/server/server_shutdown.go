package server

import (
	"context"
	"net/http"
	"os"
	ossignal "os/signal"
	"syscall"
	"time"

	"GOSpeak/internal/cluster"
	"GOSpeak/internal/logger"
	"GOSpeak/internal/plugin"
	"GOSpeak/internal/redis"
	"GOSpeak/internal/service"
	"GOSpeak/internal/signal"
	"GOSpeak/internal/ws"
)

type shutdownDeps struct {
	srv             *http.Server
	signalHub       *signal.Hub
	wsUpgrader      *ws.Upgrader
	pluginReg       *plugin.Registry
	clusterStop     func()
	stopLeaderRenew func()
	agentLeaderLock *cluster.NATSLeaderLock
	leaderFence     *service.LeaderFenceService
	instanceID      string
	closeEventBus   func()
}

func runGracefulShutdown(deps shutdownDeps) {
	quit := make(chan os.Signal, 1)
	ossignal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.WithComponent("Server").Info("shutting down...")

	// 1) stop accepting HTTP first so in-flight handlers can still emit signal
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := deps.srv.Shutdown(ctx); err != nil {
		logger.WithComponent("HTTP").Errorf("shutdown error: %v", err)
	}

	// 2) stop membership heartbeat, then close websocket connections
	deps.signalHub.StopMembershipHeartbeat()
	if deps.wsUpgrader != nil && deps.wsUpgrader.Fanout() != nil {
		deps.wsUpgrader.Fanout().CloseAll()
		logger.WithComponent("WS").Info("websocket connections closed")
	}

	deps.pluginReg.StopAll(context.Background())
	deps.pluginReg.StopAll(ctx)
	logger.WithComponent("Plugin").Info("plugins stopped")

	if deps.clusterStop != nil {
		deps.clusterStop()
		logger.WithComponent("Cluster").Info("cluster runtime stopped")
	}
	if deps.stopLeaderRenew != nil {
		deps.stopLeaderRenew()
	}
	if deps.leaderFence != nil {
		deps.leaderFence.Deactivate()
	}
	if deps.agentLeaderLock != nil {
		if err := deps.agentLeaderLock.Release(deps.instanceID); err != nil {
			logger.WithComponent("Cluster").Warnf("release agent leader lock failed: %v", err)
		}
		logger.WithComponent("Cluster").Info("agent leader lock released")
	}

	// 3) close event bus last
	redis.StopKeyRotationLoop()
	logger.WithComponent("EventBus").Info("closing event bus")
	deps.closeEventBus()
	logger.WithComponent("EventBus").Info("closed")
}
