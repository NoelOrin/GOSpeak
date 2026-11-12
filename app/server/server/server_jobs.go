package server

import (
	"GOSpeak/internal/bus"
	"GOSpeak/internal/jobs"
	"GOSpeak/internal/logger"
	"GOSpeak/internal/service"
	"GOSpeak/internal/signal"
)

func startJobConsumers(q *bus.JobQueue, instanceID string, hub *signal.Hub, messageSvc *service.MessageService, conversationSvc *service.ConversationService) {
	if q == nil {
		return
	}
	hub.SetCleanupPublisher(q)
	messageSvc.SetJobQueue(q)
	messageSvc.SetSyncWriteAllowed(true)
	conversationSvc.SetJobQueue(q)
	conversationSvc.SetSyncWriteAllowed(true)

	handler := func(job bus.JobEnvelope) error {
		return jobs.Handle(job, jobs.Deps{Cleaner: hub, Chat: messageSvc, Private: conversationSvc})
	}
	if _, err := q.Consume(instanceID, handler); err != nil {
		logger.WithComponent("JobQueue").Errorf("consumer failed: %v", err)
		return
	}
	logger.WithComponent("JobQueue").Infof("consumer started instance=%s", instanceID)
}
