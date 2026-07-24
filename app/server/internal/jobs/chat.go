package jobs

// ChatPersister is the interface for persisting and mutating chat messages
// from async job queue handlers. Implemented by *service.MessageService.
type ChatPersister interface {
	PersistFromJob(payload []byte) error
	MutateFromJob(payload []byte) error
}
