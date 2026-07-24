package jobs

import (
	"encoding/json"
	"log"

	"GOSpeak/internal/bus"
	"GOSpeak/internal/signal"
)

// StreamRegistrar is implemented by signal.Hub for SRS stream registry.
type StreamRegistrar interface {
	RegisterStream(stream string)
	UnregisterStream(stream string)
}

// SFUCleaner performs SFU-side leave cleanup.
type SFUCleaner interface {
	CleanupParticipant(room, identity string, deleteRoom bool)
}

// Handle dispatches a JobEnvelope to the appropriate handler.
func Handle(job bus.JobEnvelope, hub StreamRegistrar, cleaner SFUCleaner, chat ChatPersister) error {
	switch job.Type {
	case "srs":
		return handleSRS(job.Payload, hub)
	case "livekit":
		return handleLiveKit(job.Payload, cleaner)
	case "sfu_cleanup":
		return handleCleanup(job.Payload, cleaner)
	case "chat.persist":
		if chat == nil {
			return nil
		}
		return chat.PersistFromJob(job.Payload)
	case "chat.mutate":
		if chat == nil {
			return nil
		}
		return chat.MutateFromJob(job.Payload)
	default:
		log.Printf("[Jobs] ignore unknown type=%s", job.Type)
		return nil
	}
}

func handleSRS(raw json.RawMessage, hub StreamRegistrar) error {
	if hub == nil {
		return nil
	}
	var p struct {
		Action string `json:"action"`
		Stream string `json:"stream"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	switch p.Action {
	case "on_publish":
		hub.RegisterStream(p.Stream)
	case "on_unpublish":
		hub.UnregisterStream(p.Stream)
	}
	return nil
}

func handleLiveKit(raw json.RawMessage, cleaner SFUCleaner) error {
	var event struct {
		Event string `json:"event"`
		Room  struct {
			Name string `json:"name"`
		} `json:"room"`
		Participant struct {
			Identity string `json:"identity"`
		} `json:"participant"`
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		return err
	}
	switch event.Event {
	case "participant_left", "participant_disconnected":
		if cleaner != nil && event.Room.Name != "" && event.Participant.Identity != "" {
			cleaner.CleanupParticipant(event.Room.Name, event.Participant.Identity, false)
		}
	case "room_finished":
		if cleaner != nil && event.Room.Name != "" {
			cleaner.CleanupParticipant(event.Room.Name, "", true)
		}
	default:
		log.Printf("[Jobs] livekit event=%s (no-op)", event.Event)
	}
	return nil
}

func handleCleanup(raw json.RawMessage, cleaner SFUCleaner) error {
	if cleaner == nil {
		return nil
	}
	var p struct {
		Room       string `json:"room"`
		Identity   string `json:"identity"`
		DeleteRoom bool   `json:"delete_room"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	cleaner.CleanupParticipant(p.Room, p.Identity, p.DeleteRoom)
	return nil
}

// Ensure signal.Hub can be used without import cycle helpers elsewhere.
var _ StreamRegistrar = (*signal.Hub)(nil)
