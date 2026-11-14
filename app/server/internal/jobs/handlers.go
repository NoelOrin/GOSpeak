package jobs

import (
	"encoding/json"
	"log"

	"GOSpeak/internal/bus"
)

// SFUCleaner performs SFU-side leave cleanup.
type SFUCleaner interface {
	CleanupParticipant(room, identity string, deleteRoom bool)
}

// PrivateChatPersister persists private messages from durable jobs.
type PrivateChatPersister interface {
	PersistPrivateFromJob(payload []byte) error
}

// Deps aggregates the job-side capabilities in one struct so callers do not
// pass the same Hub three times with positional parameters.
type Deps struct {
	Cleaner SFUCleaner
	Chat    ChatPersister
	Private PrivateChatPersister
}

// Handle dispatches a JobEnvelope to the appropriate handler.
func Handle(job bus.JobEnvelope, deps Deps) error {
	switch job.Type {
	case "livekit":
		return handleLiveKit(job.Payload, deps.Cleaner)
	case "sfu_cleanup":
		return handleCleanup(job.Payload, deps.Cleaner)
	case "chat.persist":
		if deps.Chat == nil {
			return nil
		}
		return deps.Chat.PersistFromJob(job.Payload)
	case "chat.mutate":
		if deps.Chat == nil {
			return nil
		}
		return deps.Chat.MutateFromJob(job.Payload)
	case "chat.private.persist":
		if deps.Private == nil {
			return nil
		}
		return deps.Private.PersistPrivateFromJob(job.Payload)
	default:
		log.Printf("[Jobs] ignore unknown type=%s", job.Type)
		return nil
	}
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
