package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"sort"
	"sync"
	"time"
	"unicode/utf8"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/oklog/ulid/v2"
)

const (
	EventMessageNew  = "message:new"
	MaxMessageRunes  = 500
	DefaultListLimit = 50
	MaxListLimit     = 100

	asyncWriteRetries   = 3
	asyncWriteRetryBase = 100 * time.Millisecond
	writeQueueCap       = 4096
	dLRetryInterval     = 30 * time.Second
	dLMaxRetries        = 10
)

type MessageEventBus interface {
	PublishRoom(ctx context.Context, room, event string, payload interface{}) error
}

type MessageSendInput struct {
	RoomKey        string
	SenderIdentity string
	SenderDisplay  string
	SenderRole     string
	Content        string
	ReplyToID      string
	TargetIdentity string
}

type MessageDTO struct {
	ID              string `json:"id"`
	Room            string `json:"room"`
	Content         string `json:"content"`
	ReplyToID       string `json:"replyTo,omitempty"`
	SenderIdentity  string `json:"senderIdentity"`
	SenderDisplay   string `json:"senderDisplay"`
	SenderRole      string `json:"senderRole"`
	Timestamp       int64  `json:"timestamp"`
	ConversationID  string `json:"conversationId,omitempty"`
	TargetIdentity  string `json:"targetIdentity,omitempty"`
}

type MessageListResult struct {
	Messages   []MessageDTO `json:"messages"`
	NextCursor string       `json:"nextCursor,omitempty"`
}

type asyncJob struct {
	msg  *model.Message
	dto  MessageDTO
	room string
}

type dLEntry struct {
	id      string
	msg     *model.Message
	retries int
}

type MessageService struct {
	repo            *repository.MessageRepository
	conversationRepo *repository.ConversationRepository
	bus             MessageEventBus

	asyncWG  *sync.WaitGroup
	writeCh  chan *asyncJob
	deadCh   chan *model.Message
	deadMu   sync.Mutex
	deadMap  map[string]*dLEntry
	done     chan struct{}
	initOnce sync.Once
}

func NewMessageService(repo *repository.MessageRepository, convRepo *repository.ConversationRepository, bus MessageEventBus) *MessageService {
	s := &MessageService{repo: repo, conversationRepo: convRepo, bus: bus}
	s.initOnce.Do(func() {
		s.writeCh = make(chan *asyncJob, writeQueueCap)
		s.deadCh = make(chan *model.Message, 128)
		s.deadMap = make(map[string]*dLEntry)
		s.done = make(chan struct{})
		go s.writeWorker()
		go s.dlWorker()
	})
	return s
}

// SetAsyncWG sets a WaitGroup for test synchronization.
func (s *MessageService) SetAsyncWG(wg *sync.WaitGroup) {
	s.asyncWG = wg
}

func (s *MessageService) Send(ctx context.Context, in MessageSendInput) (*MessageDTO, error) {
	if in.TargetIdentity != "" {
		return s.SendDirect(ctx, in)
	}
	if in.RoomKey == "" || in.SenderIdentity == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "room and sender required")
	}
	content := in.Content
	if content == "" || utf8.RuneCountInString(content) > MaxMessageRunes {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "invalid content")
	}
	now := time.Now().UTC()
	id, err := ulid.New(ulid.Timestamp(now), rand.Reader)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR)
	}
	row := &model.Message{
		ID:             id.String(),
		RoomUUID:       strPtr(in.RoomKey),
		SenderIdentity: in.SenderIdentity,
		SenderDisplay:  in.SenderDisplay,
		SenderRole:     in.SenderRole,
		Content:        content,
		ReplyToID:      strPtrOrNil(in.ReplyToID),
		Status:         model.MessageStatusActive,
		ConversationType: model.ConversationTypeRoom,
		CreatedAt:      now,
	}
	dto := toMessageDTO(row)

	if s.asyncWG != nil {
		s.asyncWG.Add(1)
	}
	go func() {
		job := &asyncJob{msg: row, dto: dto, room: in.RoomKey}
		s.asyncSteal(ctx, job)
	}()

	return &dto, nil
}

// asyncSteal does broadcast then fastDB then write queue.
func (s *MessageService) asyncSteal(ctx context.Context, job *asyncJob) {
	if s.asyncWG != nil {
		defer s.asyncWG.Done()
	}

	// 1. broadcast
	if s.bus != nil {
		if err := s.bus.PublishRoom(ctx, job.room, EventMessageNew, job.dto); err != nil {
			log.Printf("[IM] publish message:new room=%s id=%s: %v", job.room, job.msg.ID, err)
		}
	}

	// 2. fast-path write (3 retries)
	if s.persist(job.msg) {
		return
	}

	// 3. fast failed → queue
	select {
	case s.writeCh <- job:
	default:
		log.Printf("[IM] write queue full — dropping message: room=%s id=%s", job.room, job.msg.ID)
	}
}

func (s *MessageService) persist(msg *model.Message) bool {
	for i := 0; i < asyncWriteRetries; i++ {
		if err := s.repo.Create(msg); err == nil {
			return true
		}
		if i < asyncWriteRetries-1 {
			time.Sleep(asyncWriteRetryBase * time.Duration(i+1))
		}
	}
	return false
}

func (s *MessageService) writeWorker() {
	for {
		select {
		case <-s.done:
			return
		case job := <-s.writeCh:
			if s.persist(job.msg) {
				continue
			}
			// 一次直接写 DB（不再 sleep 重试，留给 DLQ 的 30s 定时器处理）
			if err := s.repo.Create(job.msg); err == nil {
				continue
			}
			log.Printf("[IM] write worker exhausted: room=%s id=%s", job.room, job.msg.ID)
			select {
			case s.deadCh <- job.msg:
			default:
				log.Printf("[IM] DLQ full — dropping: room=%s id=%s", job.room, job.msg.ID)
			}
		}
	}
}

func (s *MessageService) dlWorker() {
	tick := time.NewTicker(dLRetryInterval)
	defer tick.Stop()
	for {
		select {
		case <-s.done:
			return
		case msg := <-s.deadCh:
			s.deadMu.Lock()
			s.deadMap[msg.ID] = &dLEntry{msg: msg}
			s.deadMu.Unlock()
		case <-tick.C:
			s.retryDead()
		}
	}
}

func (s *MessageService) retryDead() {
	// copy out → unlock → retry (no lock hold during DB write)
	s.deadMu.Lock()
	retry := make([]*dLEntry, 0, len(s.deadMap))
	for id, entry := range s.deadMap {
		entry.id = id
		retry = append(retry, entry)
	}
	s.deadMu.Unlock()

	var done []string
	for _, entry := range retry {
		if s.persist(entry.msg) {
			done = append(done, entry.id)
			continue
		}
		entry.retries++
		if entry.retries >= dLMaxRetries {
			log.Printf("[IM] dead-letter max retries: %s", entry.id)
			done = append(done, entry.id)
			continue
		}
		// still alive — re-add
		s.deadMu.Lock()
		s.deadMap[entry.id] = entry
		s.deadMu.Unlock()
	}
	for _, id := range done {
		s.deadMu.Lock()
		delete(s.deadMap, id)
		s.deadMu.Unlock()
	}
	if n := len(s.deadMap); n > 0 {
		log.Printf("[IM] dead-letter pending: %d", n)
	}
}

// Shutdown closes workers.
func (s *MessageService) Shutdown() {
	if s.done != nil {
		close(s.done)
	}
}

func (s *MessageService) List(_ context.Context, roomKey, before string, limit int) (*MessageListResult, error) {
	if roomKey == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "room required")
	}
	if limit <= 0 {
		limit = DefaultListLimit
	}
	if limit > MaxListLimit {
		limit = MaxListLimit
	}
	rows, err := s.repo.ListByRoom(roomKey, before, limit)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR)
	}
	out := &MessageListResult{Messages: make([]MessageDTO, 0, len(rows))}
	for i := range rows {
		out.Messages = append(out.Messages, toMessageDTO(&rows[i]))
	}
	if len(rows) == limit {
		out.NextCursor = rows[0].ID
	}
	return out, nil
}

// generateConversationID produces a deterministic conversation ID from two identities.
// The identities are sorted lexicographically so both sides compute the same ID.
func generateConversationID(a, b string) string {
	pair := []string{a, b}
	sort.Strings(pair)
	h := sha256.Sum256([]byte(pair[0] + ":" + pair[1]))
	return "direct:" + hex.EncodeToString(h[:8])
}

// SendDirect creates a direct (private) message between two users.
// The message is persisted synchronously (lightweight single-row write),
// and the conversation participant row is upserted with summary + unread.
func (s *MessageService) SendDirect(ctx context.Context, in MessageSendInput) (*MessageDTO, error) {
	if in.SenderIdentity == "" || in.TargetIdentity == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "sender and target required")
	}
	if in.SenderIdentity == in.TargetIdentity {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "cannot send to self")
	}
	content := in.Content
	if content == "" || utf8.RuneCountInString(content) > MaxMessageRunes {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "invalid content")
	}

	convID := generateConversationID(in.SenderIdentity, in.TargetIdentity)

	now := time.Now().UTC()
	id, err := ulid.New(ulid.Timestamp(now), rand.Reader)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR)
	}

	row := &model.Message{
		ID:               id.String(),
		SenderIdentity:   in.SenderIdentity,
		SenderDisplay:    in.SenderDisplay,
		SenderRole:       in.SenderRole,
		Content:          content,
		ReplyToID:        strPtrOrNil(in.ReplyToID),
		Status:           model.MessageStatusActive,
		ConversationType: model.ConversationTypeDirect,
		ConversationID:   &convID,
		TargetIdentity:   &in.TargetIdentity,
		CreatedAt:        now,
	}

	// Synchronous persist — direct messages are low-volume; no need for async queue.
	if err := s.repo.Create(row); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	// Upsert conversation participant row + update summary + increment unread
	if s.conversationRepo != nil {
		// Sorted identities for the participant row
		a, b := in.SenderIdentity, in.TargetIdentity
		if a > b {
			a, b = b, a
		}
		cp := &model.ConversationParticipant{
			ConversationID:      convID,
			IdentityA:          a,
			IdentityB:          b,
			LastMessageID:      strPtrOrNil(id.String()),
			LastContent:        truncateContent(content),
			LastSenderIdentity: in.SenderIdentity,
			LastMessageAt:      &now,
		}
		if err := s.conversationRepo.Upsert(cp); err != nil {
			log.Printf("[IM] conversation upsert failed conv=%s: %v", convID, err)
		}
		if err := s.conversationRepo.IncrementUnread(convID, in.SenderIdentity); err != nil {
			log.Printf("[IM] increment unread failed conv=%s: %v", convID, err)
		}
	}

	dto := toDirectMessageDTO(row)

	// Broadcast to both sender and target via event bus
	if s.bus != nil {
		if err := s.bus.PublishRoom(ctx, "__user:"+in.SenderIdentity, EventMessageNew, dto); err != nil {
			log.Printf("[IM] publish private:new sender=%s: %v", in.SenderIdentity, err)
		}
		if err := s.bus.PublishRoom(ctx, "__user:"+in.TargetIdentity, EventMessageNew, dto); err != nil {
			log.Printf("[IM] publish private:new target=%s: %v", in.TargetIdentity, err)
		}
	}

	return &dto, nil
}

func truncateContent(s string) string {
	runes := []rune(s)
	if len(runes) > 200 {
		return string(runes[:200])
	}
	return s
}

func toDirectMessageDTO(m *model.Message) MessageDTO {
	dto := toMessageDTO(m)
	if m.ConversationID != nil {
		dto.ConversationID = *m.ConversationID
	}
	if m.TargetIdentity != nil {
		dto.TargetIdentity = *m.TargetIdentity
	}
	return dto
}

func toMessageDTO(m *model.Message) MessageDTO {
	return MessageDTO{
		ID:             m.ID,
		Room:           ptrStr(m.RoomUUID),
		Content:        m.Content,
		ReplyToID:      ptrStr(m.ReplyToID),
		SenderIdentity: m.SenderIdentity,
		SenderDisplay:  m.SenderDisplay,
		SenderRole:     m.SenderRole,
		Timestamp:      m.CreatedAt.UnixMilli(),
	}
}

// strPtr returns a pointer to s.
func strPtr(s string) *string { return &s }

// strPtrOrNil returns nil for empty string, otherwise a pointer to s.
func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ptrStr dereferences a *string safely, returning "" for nil.
func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
