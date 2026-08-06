package server

import (
	"GOSpeak/internal/bus"
	"GOSpeak/internal/cluster"
	"GOSpeak/internal/config"
	"GOSpeak/internal/jobs"
	"GOSpeak/internal/logger"
	"GOSpeak/internal/model"
	"GOSpeak/internal/service"
	"GOSpeak/internal/signal"
	"encoding/json"
)

func startJobConsumers(cfg *config.Config, q *bus.JobQueue, instanceID string, hub *signal.Hub, messageSvc *service.MessageService, conversationSvc *service.ConversationService, clusterSvc *service.ClusterService, localNodeID string) {
	if q == nil {
		return
	}
	hub.SetCleanupPublisher(q)
	messageSvc.SetJobQueue(q)
	messageSvc.SetSyncWriteAllowed(cfg.IsAgent())
	conversationSvc.SetJobQueue(q)
	conversationSvc.SetSyncWriteAllowed(cfg.IsAgent())
	clusterSvc.SetControlQueue(q)

	handler := func(job bus.JobEnvelope) error {
		if job.Type == "cluster.control" {
			var cmd cluster.ControlCommand
			if err := json.Unmarshal(job.Payload, &cmd); err != nil {
				return err
			}
			if cmd.NodeID != "" && cmd.NodeID != localNodeID {
				// 定向命令不属于本节点：确认跳过，避免无限重投。
				return nil
			}
		}
		return jobs.Handle(job, jobs.Deps{Hub: hub, Cleaner: hub, Chat: messageSvc, Private: conversationSvc, Control: hub})
	}

	switch {
	case q.RoleAware() && cfg.ClusterRole == model.ClusterRoleWorker:
		if _, err := q.ConsumeRuntime(instanceID, handler); err != nil {
			logger.WithComponent("JobQueue").Errorf("runtime consumer failed: %v", err)
			return
		}
	case q.RoleAware() && cfg.ClusterRole == model.ClusterRoleAgent:
		if _, err := q.ConsumeChat(instanceID, handler); err != nil {
			logger.WithComponent("JobQueue").Errorf("chat consumer failed: %v", err)
			return
		}
	default:
		if _, err := q.Consume(instanceID, handler); err != nil {
			logger.WithComponent("JobQueue").Errorf("consumer failed: %v", err)
			return
		}
	}
	logger.WithComponent("JobQueue").Infof("consumer started instance=%s role=%s", instanceID, cfg.ClusterRole)
}
