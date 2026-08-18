package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"GOSpeak/internal/bus"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ─── Fakes ───

type fakeBus struct {
	mu    sync.Mutex
	calls []string // event names recorded
	rooms []string // room keys recorded
	err   error    // if set, PublishRoom returns this error
}

func (f *fakeBus) PublishRoom(_ context.Context, room, event string, payload interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, event)
	f.rooms = append(f.rooms, room)
	return nil
}

func (f *fakeBus) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = nil
	f.rooms = nil
	f.err = nil
}

type fakeQueue struct {
	mu   sync.Mutex
	jobs []bus.JobEnvelope
	err  error // if set, Publish returns this error
}

func (q *fakeQueue) Publish(_ context.Context, job bus.JobEnvelope) error {
	if q.err != nil {
		return q.err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.jobs = append(q.jobs, job)
	return nil
}

func (q *fakeQueue) reset() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.jobs = nil
	q.err = nil
}

// testUserRepo records user lookup call counts for enrichAuthorInfo tests.
type testUserRepo struct {
	getByNameCalls  int
	getByNamesCalls int
	users           map[string]*model.User
}

func (r *testUserRepo) GetByName(name string) (*model.User, error) {
	r.getByNameCalls++
	user, ok := r.users[name]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return user, nil
}

func (r *testUserRepo) GetByNames(names []string) (map[string]*model.User, error) {
	r.getByNamesCalls++
	users := make(map[string]*model.User, len(names))
	for _, name := range names {
		if user, ok := r.users[name]; ok {
			users[name] = user
		}
	}
	return users, nil
}

type userRepoCallCount struct {
	GetByName  int
	GetByNames int
}

func newTestMessageService(t *testing.T) *MessageService {
	t.Helper()
	svc := NewMessageService(nil, nil, nil)
	svc.SetUserRepo(&testUserRepo{
		users: map[string]*model.User{
			"alice": {Name: "alice", DisplayName: "Alice"},
			"bob":   {Name: "bob", DisplayName: "Bob"},
		},
	})
	return svc
}

func testMessageRepoCallCount(t *testing.T, svc *MessageService) userRepoCallCount {
	t.Helper()
	repo, ok := svc.userRepo.(*testUserRepo)
	if !ok {
		t.Fatalf("expected *testUserRepo, got %T", svc.userRepo)
	}
	return userRepoCallCount{GetByName: repo.getByNameCalls, GetByNames: repo.getByNamesCalls}
}

// memRoomRepo is an in-memory room store implementing roomByUUID.

type allowAllDomainMembers struct{}

func (allowAllDomainMembers) IsMember(_, _ string) bool { return true }

type fakeDomainChecker struct {
	members map[string]bool
}

func (f fakeDomainChecker) IsMember(_, user string) bool {
	return f.members[user]
}

func testActor(identity string) MessageActor {
	return MessageActor{Identity: identity, UserUUID: identity}
}

func testActorWithUUID(identity, userUUID string) MessageActor {
	return MessageActor{Identity: identity, UserUUID: userUUID}
}

type memRoomRepo struct {
	mu    sync.Mutex
	rooms map[string]*model.Room
}

func (r *memRoomRepo) GetByUUID(uuid string) (*model.Room, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	room, ok := r.rooms[uuid]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return room, nil
}

// newMemRoomRepo creates a memRoomRepo with pre-seeded text and voice rooms.
func newMemRoomRepo() *memRoomRepo {
	textUUID := uuid.New().String()
	voiceUUID := uuid.New().String()
	return &memRoomRepo{
		rooms: map[string]*model.Room{
			textUUID: {
				UUID: textUUID,
				Name: "test-text-room",
				Type: model.RoomTypeText,
			},
			voiceUUID: {
				UUID: voiceUUID,
				Name: "test-voice-room",
				Type: model.RoomTypeVoice,
			},
		},
	}
}

// addDomainTextRoom seeds a domain-scoped text room and returns its UUID.
func addDomainTextRoom(repo *memRoomRepo, name, domainUUID string) string {
	roomUUID := uuid.New().String()
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.rooms[roomUUID] = &model.Room{
		UUID:       roomUUID,
		Name:       name,
		Type:       model.RoomTypeText,
		DomainUUID: domainUUID,
	}
	return roomUUID
}

// ─── Test Setup ───

// setupMessageServiceTest creates a MessageService with in-memory SQLite-backed
// MessageRepository and fake bus/queue. Returns the service, fakes, and the
// pre-seeded text room UUID.
func setupMessageServiceTest(t *testing.T) (*MessageService, *fakeBus, *fakeQueue, *memRoomRepo, string) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	if err := db.AutoMigrate(&model.Message{}, &model.MessageReaction{}, &model.MessageMention{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	msgRepo := repository.NewMessageRepository(db)
	roomRepo := newMemRoomRepo()

	bus := &fakeBus{}
	queue := &fakeQueue{}

	svc := NewMessageService(msgRepo, roomRepo, allowAllDomainMembers{})
	// Do NOT set bus/queue by default — tests that need them wire them explicitly.
	// Default mode is sync-persist so chained operations (Send→Edit, Send→Delete)
	// find messages in DB.

	// Find the text room UUID
	var textUUID string
	for uuid, r := range roomRepo.rooms {
		if r.Type == model.RoomTypeText {
			textUUID = uuid
			break
		}
	}

	return svc, bus, queue, roomRepo, textUUID
}

// mustFindVoiceUUID returns the pre-seeded voice room UUID.
func mustFindVoiceUUID(r *memRoomRepo) string {
	for uuid, room := range r.rooms {
		if room.Type == model.RoomTypeVoice {
			return uuid
		}
	}
	return ""
}

func setupRestrictedMessageServiceTest(t *testing.T, members map[string]bool) (*MessageService, *gorm.DB, *memRoomRepo, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	if err := db.AutoMigrate(&model.Message{}, &model.MessageReaction{}, &model.MessageMention{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	msgRepo := repository.NewMessageRepository(db)
	roomRepo := newMemRoomRepo()
	svc := NewMessageService(msgRepo, roomRepo, fakeDomainChecker{members: members})
	var textUUID string
	for uuid, r := range roomRepo.rooms {
		if r.Type == model.RoomTypeText {
			textUUID = uuid
			break
		}
	}
	return svc, db, roomRepo, textUUID
}

// ─── Helpers ───

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertErrorCode(t *testing.T, err error, code pkg.ErrCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %d, got nil", code)
	}
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *pkg.AppError, got %T: %v", err, err)
	}
	if appErr.Code != code {
		t.Fatalf("expected error code %d, got %d: %s", code, appErr.Code, appErr.Message)
	}
}

// ─── Author Enrichment Tests ───

func TestEnrichAuthorInfo_Batch(t *testing.T) {
	svc := newTestMessageService(t)
	items := []MessageDTO{{AuthorID: "alice"}, {AuthorID: "bob"}, {AuthorID: "alice"}}
	svc.enrichAuthorInfo(items)
	if items[0].AuthorName == "" || items[1].AuthorName == "" {
		t.Fatal("expected author names enriched")
	}
	if calls := testMessageRepoCallCount(t, svc); calls.GetByName != 0 || calls.GetByNames != 1 {
		t.Fatalf("expected one batch call, got %+v", calls)
	}
}

// ─── Send Tests ───

func TestMessageService_RejectsNonDomainMemberForAllEndpoints(t *testing.T) {
	svc, db, roomRepo, textUUID := setupRestrictedMessageServiceTest(t, map[string]bool{"uuid-alice": true})
	room := roomRepo.rooms[textUUID]
	room.DomainUUID = "domain-a"

	if _, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "member message", "", "", nil); err != nil {
		t.Fatalf("member send should succeed: %v", err)
	}

	now := time.Now().UTC()
	msg := &model.Message{
		UUID:      uuid.New().String(),
		RoomUUID:  textUUID,
		AuthorID:  "eve",
		Content:   "owned by non-member",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(msg).Error; err != nil {
		t.Fatal(err)
	}

	_, sendErr := svc.Send(textUUID, testActorWithUUID("eve", "uuid-eve"), "hello", "", "", nil)
	assertErrorCode(t, sendErr, pkg.FORBIDDEN)
	_, editErr := svc.Edit(textUUID, msg.UUID, testActorWithUUID("eve", "uuid-eve"), "changed")
	assertErrorCode(t, editErr, pkg.FORBIDDEN)
	deleteErr := svc.Delete(textUUID, msg.UUID, testActorWithUUID("eve", "uuid-eve"), true)
	assertErrorCode(t, deleteErr, pkg.FORBIDDEN)
	reactErr := svc.React(textUUID, msg.UUID, testActorWithUUID("eve", "uuid-eve"), "+1")
	assertErrorCode(t, reactErr, pkg.FORBIDDEN)
	unreactErr := svc.Unreact(textUUID, msg.UUID, testActorWithUUID("eve", "uuid-eve"), "+1")
	assertErrorCode(t, unreactErr, pkg.FORBIDDEN)
	_, _, _, listErr := svc.ListHistory(textUUID, testActorWithUUID("eve", "uuid-eve"), "", 50, "")
	assertErrorCode(t, listErr, pkg.FORBIDDEN)
	_, searchErr := svc.Search(textUUID, testActorWithUUID("eve", "uuid-eve"), "owned", "")
	assertErrorCode(t, searchErr, pkg.FORBIDDEN)

	if _, _, _, err := svc.ListHistory(textUUID, testActorWithUUID("alice", "uuid-alice"), "", 50, ""); err != nil {
		t.Fatalf("member history should succeed: %v", err)
	}
}

func TestMessageService_UsesUserUUIDForDomainMembership(t *testing.T) {
	svc, _, roomRepo, textUUID := setupRestrictedMessageServiceTest(t, map[string]bool{"uuid-alice": true})
	room := roomRepo.rooms[textUUID]
	room.DomainUUID = "domain-a"

	if _, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "ok", "", "", nil); err != nil {
		t.Fatalf("member with matching user_uuid should succeed: %v", err)
	}
	_, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-eve"), "forbidden", "", "", nil)
	assertErrorCode(t, err, pkg.FORBIDDEN)
	_, err = svc.Send(textUUID, MessageActor{Identity: "alice"}, "missing uuid", "", "", nil)
	assertErrorCode(t, err, pkg.TOKEN_WRONG)
}

func TestMessageService_Send_Success(t *testing.T) {
	svc, bus, queue, _, textUUID := setupMessageServiceTest(t)
	t.Cleanup(func() { bus.reset(); queue.reset() })

	svc.SetEventBus(bus)
	svc.SetJobQueue(queue)

	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "Hello, world!", "", "", nil)
	assertNoError(t, err)

	if dto == nil {
		t.Fatal("expected non-nil DTO")
	}
	if dto.RoomUUID != textUUID {
		t.Errorf("expected room_uuid %s, got %s", textUUID, dto.RoomUUID)
	}
	if dto.AuthorID != "alice" {
		t.Errorf("expected author_id alice, got %s", dto.AuthorID)
	}
	if dto.AuthorUUID != "uuid-alice" {
		t.Errorf("expected author_uuid uuid-alice, got %s", dto.AuthorUUID)
	}
	if dto.Content != "Hello, world!" {
		t.Errorf("expected content 'Hello, world!', got %s", dto.Content)
	}
	if dto.Deleted {
		t.Error("expected deleted=false")
	}
	if dto.UUID == "" {
		t.Error("expected non-empty UUID")
	}
	if dto.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}

	// Bus should have received "message:created"
	bus.mu.Lock()
	busCalls := len(bus.calls)
	bus.mu.Unlock()
	if busCalls != 1 {
		t.Errorf("expected 1 bus call, got %d", busCalls)
	}
	if len(bus.calls) > 0 && bus.calls[0] != "message:created" {
		t.Errorf("expected bus event 'message:created', got %s", bus.calls[0])
	}

	// Queue should have received a chat.persist job
	queue.mu.Lock()
	queueJobs := len(queue.jobs)
	queue.mu.Unlock()
	if queueJobs != 1 {
		t.Errorf("expected 1 queue job, got %d", queueJobs)
	}
	if queueJobs > 0 {
		job := queue.jobs[0]
		if job.Type != "chat.persist" {
			t.Errorf("expected job type 'chat.persist', got %s", job.Type)
		}
		if job.ID != dto.UUID {
			t.Errorf("expected job ID %s, got %s", dto.UUID, job.ID)
		}
	}
}

func TestMessageService_Send_WithMentions(t *testing.T) {
	svc, bus, queue, _, textUUID := setupMessageServiceTest(t)
	t.Cleanup(func() { bus.reset(); queue.reset() })

	svc.SetEventBus(bus)
	svc.SetJobQueue(queue)

	mentions := []string{"bob", "charlie"}
	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "Hey @bob and @charlie!", "", "", mentions)
	assertNoError(t, err)

	if len(dto.Mentions) != 2 {
		t.Errorf("expected 2 mentions, got %d", len(dto.Mentions))
	}

	// Verify mentions are in the job payload
	if len(queue.jobs) > 0 {
		var payload struct {
			Mentions []string `json:"mentions"`
		}
		if err := json.Unmarshal(queue.jobs[0].Payload, &payload); err != nil {
			t.Fatalf("failed to unmarshal job payload: %v", err)
		}
		if len(payload.Mentions) != 2 {
			t.Errorf("expected 2 mentions in job payload, got %d", len(payload.Mentions))
		}
	}
}

func TestMessageService_Send_WithClientNonce(t *testing.T) {
	svc, bus, queue, _, textUUID := setupMessageServiceTest(t)
	t.Cleanup(func() { bus.reset(); queue.reset() })

	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "Hello", "", "nonce-123", nil)
	assertNoError(t, err)

	if dto.ClientNonce != "nonce-123" {
		t.Errorf("expected client_nonce 'nonce-123', got %s", dto.ClientNonce)
	}
}

func TestMessageService_Send_WithReplyTo(t *testing.T) {
	svc, bus, queue, _, textUUID := setupMessageServiceTest(t)
	t.Cleanup(func() { bus.reset(); queue.reset() })

	// First send a message to use as parent
	parent, err := svc.Send(textUUID, testActorWithUUID("bob", "uuid-bob"), "Parent message", "", "", nil)
	assertNoError(t, err)

	// Reset fakes to isolate the reply send
	bus.reset()
	queue.reset()

	// Send a reply
	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "Reply message", parent.UUID, "", nil)
	assertNoError(t, err)

	if dto.ReplyTo != parent.UUID {
		t.Errorf("expected reply_to %s, got %s", parent.UUID, dto.ReplyTo)
	}
}

func TestMessageService_Send_InvalidReplyTo(t *testing.T) {
	svc, bus, queue, _, textUUID := setupMessageServiceTest(t)
	t.Cleanup(func() { bus.reset(); queue.reset() })

	_, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "Reply message", "nonexistent-uuid", "", nil)
	assertErrorCode(t, err, pkg.INVALID_PARAMS)
}

func TestMessageService_Send_EmptyContent(t *testing.T) {
	svc, _, _, _, textUUID := setupMessageServiceTest(t)

	_, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "   ", "", "", nil)
	assertErrorCode(t, err, pkg.INVALID_PARAMS)
}

func TestMessageService_Send_RejectsEmptyActor(t *testing.T) {
	svc, _, _, _, textUUID := setupMessageServiceTest(t)
	_, err := svc.Send(textUUID, MessageActor{}, "hello", "", "", nil)
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) || appErr.Code != pkg.TOKEN_WRONG {
		t.Fatalf("want TOKEN_WRONG, got %v", err)
	}
}

func TestMessageService_Send_TooLongContent(t *testing.T) {
	svc, _, _, _, textUUID := setupMessageServiceTest(t)

	long := strings.Repeat("a", MaxMessageRunes+1)
	_, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), long, "", "", nil)
	assertErrorCode(t, err, pkg.INVALID_PARAMS)
}

func TestMessageService_Send_MaxRunesBoundary(t *testing.T) {
	svc, _, _, _, textUUID := setupMessageServiceTest(t)

	// Exactly 2000 runes — should succeed
	content := strings.Repeat("a", MaxMessageRunes)
	if utf8.RuneCountInString(content) != MaxMessageRunes {
		t.Fatalf("expected %d runes, got %d", MaxMessageRunes, utf8.RuneCountInString(content))
	}
	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), content, "", "", nil)
	assertNoError(t, err)
	if dto == nil {
		t.Fatal("expected non-nil DTO")
	}

	// 2001 runes — should fail
	contentTooLong := content + "a"
	_, err = svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), contentTooLong, "", "", nil)
	assertErrorCode(t, err, pkg.INVALID_PARAMS)
}

func TestMessageService_Send_VoiceRoom(t *testing.T) {
	svc, _, _, roomRepo, _ := setupMessageServiceTest(t)
	voiceUUID := mustFindVoiceUUID(roomRepo)
	if voiceUUID == "" {
		t.Fatal("voice room not found")
	}

	_, err := svc.Send(voiceUUID, testActorWithUUID("alice", "uuid-alice"), "Hello", "", "", nil)
	assertErrorCode(t, err, pkg.FORBIDDEN)
}

func TestMessageService_Send_RoomNotFound(t *testing.T) {
	svc, _, _, _, _ := setupMessageServiceTest(t)

	_, err := svc.Send("nonexistent-uuid", testActorWithUUID("alice", "uuid-alice"), "Hello", "", "", nil)
	assertErrorCode(t, err, pkg.NOT_FOUND)
}

func TestMessageService_Send_QueueFallback(t *testing.T) {
	svc, bus, queue, _, textUUID := setupMessageServiceTest(t)
	t.Cleanup(func() { bus.reset(); queue.reset() })

	svc.SetEventBus(bus)
	svc.SetJobQueue(queue)
	// Make the queue return an error so sync fallback kicks in
	queue.err = errors.New("nats unavailable")

	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "Fallback test", "", "", nil)
	assertNoError(t, err)

	// Broadcast still happened
	if len(bus.calls) != 1 {
		t.Errorf("expected 1 bus call, got %d", len(bus.calls))
	}

	// Message should be persisted directly (sync fallback)
	msg, err := svc.msgRepo.GetByUUID(dto.UUID)
	assertNoError(t, err)
	if msg == nil || msg.Content != "Fallback test" {
		t.Errorf("expected message to be persisted, got %+v", msg)
	}
}

func TestMessageService_Send_NoBusNoQueue(t *testing.T) {
	svc, _, _, _, textUUID := setupMessageServiceTest(t)
	// Remove bus and queue to test bare mode
	svc.SetEventBus(nil)
	svc.SetJobQueue(nil)

	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "Bare mode", "", "", nil)
	assertNoError(t, err)

	// Should have been persisted synchronously
	msg, err := svc.msgRepo.GetByUUID(dto.UUID)
	assertNoError(t, err)
	if msg == nil || msg.Content != "Bare mode" {
		t.Errorf("expected message to be persisted in bare mode, got %+v", msg)
	}
}

func TestMessageService_Send_RejectsSyncWriteOnDataPlane(t *testing.T) {
	svc, bus, queue, _, textUUID := setupMessageServiceTest(t)
	t.Cleanup(func() { bus.reset(); queue.reset() })
	svc.SetEventBus(bus)
	svc.SetJobQueue(queue)
	svc.SetSyncWriteAllowed(false)
	queue.err = errors.New("nats unavailable")

	_, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "Worker must not write", "", "", nil)
	if err == nil {
		t.Fatal("expected data-plane sync-write rejection")
	}
	if len(bus.calls) != 1 {
		t.Fatalf("expected broadcast to still happen, got %d calls", len(bus.calls))
	}
}

// ─── Edit Tests ───

func TestMessageService_Edit_Success(t *testing.T) {
	svc, bus, queue, _, textUUID := setupMessageServiceTest(t)
	t.Cleanup(func() { bus.reset(); queue.reset() })

	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "Original", "", "", nil)
	assertNoError(t, err)

	bus.reset()
	queue.reset()
	svc.SetEventBus(bus)
	svc.SetJobQueue(queue)

	edited, err := svc.Edit(textUUID, dto.UUID, testActorWithUUID("alice", "uuid-alice"), "Edited content")
	assertNoError(t, err)
	if edited.Content != "Edited content" {
		t.Errorf("expected 'Edited content', got %s", edited.Content)
	}
	if edited.EditedAt == nil {
		t.Error("expected non-nil EditedAt")
	}

	// Bus got message:updated
	if len(bus.calls) != 1 || bus.calls[0] != "message:updated" {
		t.Errorf("expected bus event 'message:updated', got %v", bus.calls)
	}

	// Queue got chat.mutate
	if len(queue.jobs) != 1 || queue.jobs[0].Type != "chat.mutate" {
		t.Errorf("expected 1 chat.mutate job, got %d jobs: %+v", len(queue.jobs), queue.jobs)
	}
}

func TestMessageService_Edit_EmptyContent(t *testing.T) {
	svc, bus, queue, _, textUUID := setupMessageServiceTest(t)
	t.Cleanup(func() { bus.reset(); queue.reset() })

	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "Original", "", "", nil)
	assertNoError(t, err)

	_, err = svc.Edit(textUUID, dto.UUID, testActorWithUUID("alice", "uuid-alice"), "  ")
	assertErrorCode(t, err, pkg.INVALID_PARAMS)
}

func TestMessageService_Edit_NotAuthor(t *testing.T) {
	svc, bus, queue, _, textUUID := setupMessageServiceTest(t)
	t.Cleanup(func() { bus.reset(); queue.reset() })

	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "Original", "", "", nil)
	assertNoError(t, err)

	_, err = svc.Edit(textUUID, dto.UUID, testActorWithUUID("bob", "uuid-bob"), "Hacked content")
	assertErrorCode(t, err, pkg.FORBIDDEN)
}

func TestMessageService_EditDelete_UsesAuthorUUIDAfterRename(t *testing.T) {
	svc, _, _, _, textUUID := setupMessageServiceTest(t)

	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "Original", "", "", nil)
	assertNoError(t, err)
	if dto.AuthorUUID != "uuid-alice" {
		t.Fatalf("expected author_uuid uuid-alice, got %s", dto.AuthorUUID)
	}

	// 同一用户改名后仍可按 UUID 编辑。
	renamed := testActorWithUUID("alice-renamed", "uuid-alice")
	edited, err := svc.Edit(textUUID, dto.UUID, renamed, "Edited after rename")
	assertNoError(t, err)
	if edited.AuthorUUID != "uuid-alice" {
		t.Errorf("expected edited author_uuid uuid-alice, got %s", edited.AuthorUUID)
	}

	// 同名但不同 UUID 的用户不能冒名编辑。
	_, err = svc.Edit(textUUID, dto.UUID, testActorWithUUID("alice-renamed", "uuid-eve"), "Hacked")
	assertErrorCode(t, err, pkg.FORBIDDEN)

	// 历史查询应带回稳定 UUID。
	items, _, _, err := svc.ListHistory(textUUID, renamed, "", 50, "")
	assertNoError(t, err)
	if len(items) != 1 || items[0].AuthorUUID != "uuid-alice" {
		t.Fatalf("expected history author_uuid uuid-alice, got %+v", items)
	}

	// 删除同样按 UUID 鉴权。
	err = svc.Delete(textUUID, dto.UUID, renamed, false)
	assertNoError(t, err)
}

func TestMessageService_Edit_NotFound(t *testing.T) {
	svc, _, _, _, _ := setupMessageServiceTest(t)

	_, err := svc.Edit("room-uuid", "nonexistent-uuid", testActorWithUUID("alice", "uuid-alice"), "Content")
	assertErrorCode(t, err, pkg.NOT_FOUND)
}

func TestMessageService_Edit_DeletedMessage(t *testing.T) {
	svc, _, _, _, textUUID := setupMessageServiceTest(t)
	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "original", "", "", nil)
	assertNoError(t, err)
	assertNoError(t, svc.Delete(textUUID, dto.UUID, testActorWithUUID("alice", "uuid-alice"), false))

	_, err = svc.Edit(textUUID, dto.UUID, testActorWithUUID("alice", "uuid-alice"), "edited")
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) || appErr.Code != pkg.NOT_FOUND {
		t.Fatalf("want NOT_FOUND, got %v", err)
	}
}

func TestMessageService_Edit_QueueFallback(t *testing.T) {
	svc, bus, queue, _, textUUID := setupMessageServiceTest(t)
	t.Cleanup(func() { bus.reset(); queue.reset() })

	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "Original", "", "", nil)
	assertNoError(t, err)

	bus.reset()
	queue.err = errors.New("nats unavailable")

	_, err = svc.Edit(textUUID, dto.UUID, testActorWithUUID("alice", "uuid-alice"), "Sync edited")
	assertNoError(t, err)

	// Should be updated in DB
	msg, err := svc.msgRepo.GetByUUID(dto.UUID)
	assertNoError(t, err)
	if msg.Content != "Sync edited" {
		t.Errorf("expected content 'Sync edited', got %s", msg.Content)
	}
}

// ─── Delete Tests ───

func TestMessageService_Delete_Success(t *testing.T) {
	svc, bus, queue, _, textUUID := setupMessageServiceTest(t)
	t.Cleanup(func() { bus.reset(); queue.reset() })

	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "To be deleted", "", "", nil)
	assertNoError(t, err)

	bus.reset()
	queue.reset()
	svc.SetEventBus(bus)
	svc.SetJobQueue(queue)

	err = svc.Delete(textUUID, dto.UUID, testActorWithUUID("alice", "uuid-alice"), false)
	assertNoError(t, err)

	// Bus got message:deleted
	if len(bus.calls) != 1 || bus.calls[0] != "message:deleted" {
		t.Errorf("expected bus event 'message:deleted', got %v", bus.calls)
	}

	// Queue got chat.mutate
	if len(queue.jobs) != 1 || queue.jobs[0].Type != "chat.mutate" {
		t.Errorf("expected 1 chat.mutate job, got %d jobs", len(queue.jobs))
	}
}

func TestMessageService_Delete_OthersWithoutPermission(t *testing.T) {
	svc, _, _, _, textUUID := setupMessageServiceTest(t)

	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "My message", "", "", nil)
	assertNoError(t, err)

	err = svc.Delete(textUUID, dto.UUID, testActorWithUUID("bob", "uuid-bob"), false)
	assertErrorCode(t, err, pkg.FORBIDDEN)
}

func TestMessageService_Delete_WithPermission(t *testing.T) {
	svc, _, queue, _, textUUID := setupMessageServiceTest(t)

	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "My message", "", "", nil)
	assertNoError(t, err)

	svc.SetJobQueue(queue)
	err = svc.Delete(textUUID, dto.UUID, testActorWithUUID("moderator", "uuid-moderator"), true)
	assertNoError(t, err)

	// Queue should have a job
	if len(queue.jobs) < 1 {
		t.Error("expected at least 1 queue job")
	}
}

func TestMessageService_Delete_NotFound(t *testing.T) {
	svc, _, _, _, _ := setupMessageServiceTest(t)

	err := svc.Delete("room-uuid", "nonexistent-uuid", testActorWithUUID("alice", "uuid-alice"), false)
	assertErrorCode(t, err, pkg.NOT_FOUND)
}

func TestMessageService_Delete_QueueFallback(t *testing.T) {
	svc, bus, queue, _, textUUID := setupMessageServiceTest(t)
	t.Cleanup(func() { bus.reset(); queue.reset() })

	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "To be deleted sync", "", "", nil)
	assertNoError(t, err)

	svc.SetEventBus(bus)
	svc.SetJobQueue(queue)
	queue.err = errors.New("nats unavailable")

	err = svc.Delete(textUUID, dto.UUID, testActorWithUUID("alice", "uuid-alice"), false)
	assertNoError(t, err)

	// Broadcast still happened
	if len(bus.calls) != 1 {
		t.Errorf("expected 1 bus call after fallback, got %d", len(bus.calls))
	}

	// Message should be soft-deleted in DB
	// However, GORM filters soft-deleted by default, so GetByUUID should fail
	_, err = svc.msgRepo.GetByUUID(dto.UUID)
	if err == nil {
		t.Error("expected message to be soft-deleted (GetByUUID should fail)")
	}
}

// ─── React Tests ───

func TestMessageService_React_Success(t *testing.T) {
	svc, bus, queue, _, textUUID := setupMessageServiceTest(t)
	t.Cleanup(func() { bus.reset(); queue.reset() })

	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "Reactable", "", "", nil)
	assertNoError(t, err)

	svc.SetEventBus(bus)
	svc.SetJobQueue(queue)

	err = svc.React(textUUID, dto.UUID, testActorWithUUID("bob", "uuid-bob"), "+1")
	assertNoError(t, err)

	// Bus got message:reaction
	if len(bus.calls) != 1 || bus.calls[0] != "message:reaction" {
		t.Errorf("expected bus event 'message:reaction', got %v", bus.calls)
	}

	// Queue got chat.mutate
	if len(queue.jobs) != 1 || queue.jobs[0].Type != "chat.mutate" {
		t.Errorf("expected 1 chat.mutate job, got %d jobs", len(queue.jobs))
	}
}

func TestMessageService_React_EmptyEmoji(t *testing.T) {
	svc, _, _, _, textUUID := setupMessageServiceTest(t)

	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "Reactable", "", "", nil)
	assertNoError(t, err)

	err = svc.React(textUUID, dto.UUID, testActorWithUUID("bob", "uuid-bob"), "")
	assertErrorCode(t, err, pkg.INVALID_PARAMS)
}

func TestMessageService_React_NotFound(t *testing.T) {
	svc, _, _, _, _ := setupMessageServiceTest(t)

	err := svc.React("room-uuid", "nonexistent-uuid", testActorWithUUID("bob", "uuid-bob"), "+1")
	assertErrorCode(t, err, pkg.NOT_FOUND)
}

func TestMessageService_React_QueueFallback(t *testing.T) {
	svc, _, queue, _, textUUID := setupMessageServiceTest(t)

	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "Reactable", "", "", nil)
	assertNoError(t, err)

	queue.err = errors.New("nats unavailable")
	// Reset queue state from Send
	queue.reset()

	err = svc.React(textUUID, dto.UUID, testActorWithUUID("bob", "uuid-bob"), "+1")
	assertNoError(t, err)

	// Reaction should be persisted in DB
	reactions, err := svc.msgRepo.ListReactions([]string{dto.UUID})
	assertNoError(t, err)
	if len(reactions) != 1 {
		t.Errorf("expected 1 reaction, got %d", len(reactions))
	}
	if reactions[0].Emoji != "+1" {
		t.Errorf("expected emoji '+1', got %s", reactions[0].Emoji)
	}
	if reactions[0].UserID != "bob" {
		t.Errorf("expected user_id 'bob', got %s", reactions[0].UserID)
	}
}

// ─── Unreact Tests ───

func TestMessageService_Unreact_Success(t *testing.T) {
	svc, bus, queue, _, textUUID := setupMessageServiceTest(t)
	t.Cleanup(func() { bus.reset(); queue.reset() })

	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "Reactable", "", "", nil)
	assertNoError(t, err)

	// Add reaction first (sync, no bus/queue needed)
	err = svc.React(textUUID, dto.UUID, testActorWithUUID("bob", "uuid-bob"), "+1")
	assertNoError(t, err)

	svc.SetEventBus(bus)
	svc.SetJobQueue(queue)

	err = svc.Unreact(textUUID, dto.UUID, testActorWithUUID("bob", "uuid-bob"), "+1")
	assertNoError(t, err)

	// Bus got message:reaction
	if len(bus.calls) != 1 || bus.calls[0] != "message:reaction" {
		t.Errorf("expected bus event 'message:reaction', got %v", bus.calls)
	}

	// Queue got chat.mutate
	if len(queue.jobs) != 1 || queue.jobs[0].Type != "chat.mutate" {
		t.Errorf("expected 1 chat.mutate job, got %d jobs", len(queue.jobs))
	}
}

func TestMessageService_Unreact_EmptyEmoji(t *testing.T) {
	svc, _, _, _, textUUID := setupMessageServiceTest(t)

	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "Reactable", "", "", nil)
	assertNoError(t, err)

	err = svc.Unreact(textUUID, dto.UUID, testActorWithUUID("bob", "uuid-bob"), "")
	assertErrorCode(t, err, pkg.INVALID_PARAMS)
}

func TestMessageService_Unreact_QueueFallback(t *testing.T) {
	svc, _, queue, _, textUUID := setupMessageServiceTest(t)

	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "Reactable", "", "", nil)
	assertNoError(t, err)

	// Add reaction first
	err = svc.React(textUUID, dto.UUID, testActorWithUUID("bob", "uuid-bob"), "+1")
	assertNoError(t, err)

	queue.err = errors.New("nats unavailable")
	queue.reset()

	// Cancel the last reset's clearing by re-setting error
	queue.err = errors.New("nats unavailable")

	err = svc.Unreact(textUUID, dto.UUID, testActorWithUUID("bob", "uuid-bob"), "+1")
	assertNoError(t, err)

	// Reaction should be removed from DB
	reactions, err := svc.msgRepo.ListReactions([]string{dto.UUID})
	assertNoError(t, err)
	if len(reactions) != 0 {
		t.Errorf("expected 0 reactions after unreact, got %d", len(reactions))
	}
}

// ─── ListHistory Tests ───

func TestMessageService_ListHistory_Empty(t *testing.T) {
	svc, _, _, _, textUUID := setupMessageServiceTest(t)

	items, hasMore, nextBefore, err := svc.ListHistory(textUUID, testActorWithUUID("alice", "uuid-alice"), "", 50, "")
	assertNoError(t, err)

	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
	if hasMore {
		t.Error("expected hasMore=false for empty room")
	}
	if nextBefore != "" {
		t.Errorf("expected empty nextBefore, got %s", nextBefore)
	}
}

func TestMessageService_ListHistory_ReturnsMessages(t *testing.T) {
	svc, _, _, _, textUUID := setupMessageServiceTest(t)

	// Send 3 messages
	msg1, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "First", "", "", nil)
	assertNoError(t, err)
	_, err = svc.Send(textUUID, testActorWithUUID("bob", "uuid-bob"), "Second", "", "", nil)
	assertNoError(t, err)
	msg3, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "Third", "", "", nil)
	assertNoError(t, err)

	items, hasMore, nextBefore, err := svc.ListHistory(textUUID, testActorWithUUID("alice", "uuid-alice"), "", 100, "")
	assertNoError(t, err)

	// Items should be in ASC order (oldest first)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].UUID != msg1.UUID {
		t.Errorf("expected first item UUID %s, got %s", msg1.UUID, items[0].UUID)
	}
	if items[2].UUID != msg3.UUID {
		t.Errorf("expected last item UUID %s, got %s", msg3.UUID, items[2].UUID)
	}

	if hasMore {
		t.Error("expected hasMore=false for 3 messages with limit 100")
	}
	if nextBefore != "" {
		t.Errorf("expected empty nextBefore, got %s", nextBefore)
	}
}

func TestMessageService_ListHistory_ClampLimit(t *testing.T) {
	svc, _, _, _, textUUID := setupMessageServiceTest(t)

	// Send messages to test clamping
	for i := 0; i < 60; i++ {
		_, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "msg", "", "", nil)
		assertNoError(t, err)
	}

	// limit = 0 defaults to 100
	items, hasMore, nextBefore, err := svc.ListHistory(textUUID, testActorWithUUID("alice", "uuid-alice"), "", 0, "")
	assertNoError(t, err)
	if len(items) != 60 {
		t.Errorf("expected 60 items with default limit, got %d", len(items))
	}
	if hasMore {
		t.Error("expected hasMore=false for 60 messages")
	}
	if nextBefore != "" {
		t.Errorf("expected empty nextBefore, got %s", nextBefore)
	}

	// limit < 50 should clamp to 50
	items, hasMore, nextBefore, err = svc.ListHistory(textUUID, testActorWithUUID("alice", "uuid-alice"), "", 10, "")
	assertNoError(t, err)
	if len(items) != 50 {
		t.Errorf("expected 50 items (clamped from 10), got %d", len(items))
	}
	if !hasMore {
		t.Error("expected hasMore=true when more messages exist")
	}
	if nextBefore == "" {
		t.Error("expected non-empty nextBefore when hasMore")
	}
}

func TestMessageService_ListHistory_LimitCap(t *testing.T) {
	svc, _, _, _, textUUID := setupMessageServiceTest(t)

	// Send 250 messages
	for i := 0; i < 250; i++ {
		_, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "msg", "", "", nil)
		assertNoError(t, err)
	}

	// limit > 200 should clamp to 200
	items, hasMore, nextBefore, err := svc.ListHistory(textUUID, testActorWithUUID("alice", "uuid-alice"), "", 1000, "")
	assertNoError(t, err)
	if len(items) != 200 {
		t.Errorf("expected 200 items (capped from 1000), got %d", len(items))
	}
	if !hasMore {
		t.Error("expected hasMore=true for 250 msgs with limit 200")
	}
	if nextBefore == "" {
		t.Error("expected non-empty nextBefore when hasMore")
	}
}

func TestMessageService_ListHistory_Pagination(t *testing.T) {
	svc, _, _, _, textUUID := setupMessageServiceTest(t)

	// Send 120 messages (enough for 3 pages of 50)
	var uuids []string
	for i := 0; i < 120; i++ {
		dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "msg", "", "", nil)
		assertNoError(t, err)
		uuids = append(uuids, dto.UUID)
	}

	// First page: get 50 messages (limit clamped to 50 minimum)
	items, hasMore, nextBefore, err := svc.ListHistory(textUUID, testActorWithUUID("alice", "uuid-alice"), "", 50, "")
	assertNoError(t, err)
	if len(items) != 50 {
		t.Fatalf("expected 50 items on page 1, got %d", len(items))
	}
	if !hasMore {
		t.Error("expected hasMore=true on page 1")
	}
	if nextBefore == "" {
		t.Error("expected non-empty nextBefore on page 1")
	}

	// Second page: pass nextBefore
	items2, hasMore2, nextBefore2, err := svc.ListHistory(textUUID, testActorWithUUID("alice", "uuid-alice"), nextBefore, 50, "")
	assertNoError(t, err)
	if len(items2) != 50 {
		t.Fatalf("expected 50 items on page 2, got %d", len(items2))
	}
	if !hasMore2 {
		t.Error("expected hasMore=true on page 2")
	}

	// Third page: should get remaining 20
	items3, hasMore3, nextBefore3, err := svc.ListHistory(textUUID, testActorWithUUID("alice", "uuid-alice"), nextBefore2, 50, "")
	assertNoError(t, err)
	if len(items3) != 20 {
		t.Fatalf("expected 20 items on page 3, got %d", len(items3))
	}
	if hasMore3 {
		t.Error("expected hasMore=false on page 3")
	}
	if nextBefore3 != "" {
		t.Errorf("expected empty nextBefore on page 3, got %s", nextBefore3)
	}

	// Verify no duplicates across pages
	seen := make(map[string]bool)
	for _, item := range items {
		seen[item.UUID] = true
	}
	for _, item := range items2 {
		if seen[item.UUID] {
			t.Errorf("duplicate UUID %s across pages", item.UUID)
		}
		seen[item.UUID] = true
	}
	for _, item := range items3 {
		if seen[item.UUID] {
			t.Errorf("duplicate UUID %s across pages", item.UUID)
		}
		seen[item.UUID] = true
	}
	if len(seen) != 120 {
		t.Errorf("expected 120 unique messages across pages, got %d", len(seen))
	}
}

// ─── PersistFromJob Tests ───

func TestMessageService_PersistFromJob_Success(t *testing.T) {
	svc, _, _, _, _ := setupMessageServiceTest(t)

	now := time.Now().UTC()
	payload, err := json.Marshal(map[string]interface{}{
		"uuid":       uuid.New().String(),
		"room_uuid":  uuid.New().String(),
		"author_id":  "alice",
		"content":    "Persisted from job",
		"reply_to":   "",
		"mentions":   []string{},
		"created_at": now,
	})
	assertNoError(t, err)

	err = svc.PersistFromJob(payload)
	assertNoError(t, err)
}

func TestMessageService_PersistFromJob_WithMentions(t *testing.T) {
	svc, _, _, _, _ := setupMessageServiceTest(t)

	now := time.Now().UTC()
	payload, err := json.Marshal(map[string]interface{}{
		"uuid":       uuid.New().String(),
		"room_uuid":  uuid.New().String(),
		"author_id":  "alice",
		"content":    "Persisted with mentions",
		"reply_to":   "",
		"mentions":   []string{"bob", "charlie"},
		"created_at": now,
	})
	assertNoError(t, err)

	err = svc.PersistFromJob(payload)
	assertNoError(t, err)
}

func TestMessageService_PersistFromJob_InvalidPayload(t *testing.T) {
	svc, _, _, _, _ := setupMessageServiceTest(t)

	err := svc.PersistFromJob([]byte("invalid json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMessageService_JobWritesRejectedOnDataPlane(t *testing.T) {
	svc, _, _, _, _ := setupMessageServiceTest(t)
	svc.SetSyncWriteAllowed(false)

	now := time.Now().UTC()
	payload, err := json.Marshal(map[string]interface{}{
		"uuid":       uuid.New().String(),
		"room_uuid":  uuid.New().String(),
		"author_id":  "alice",
		"content":    "must not persist",
		"reply_to":   "",
		"mentions":   []string{},
		"created_at": now,
	})
	assertNoError(t, err)
	if err := svc.PersistFromJob(payload); err == nil {
		t.Fatal("expected data-plane persist job to be rejected")
	}

	mutatePayload, _ := json.Marshal(map[string]interface{}{
		"action":       "edit",
		"message_uuid": uuid.New().String(),
		"content":      "must not mutate",
		"room_uuid":    uuid.New().String(),
		"author_id":    "alice",
	})
	if err := svc.MutateFromJob(mutatePayload); err == nil {
		t.Fatal("expected data-plane mutate job to be rejected")
	}
}

// ─── MutateFromJob Tests ───

func TestMessageService_MutateFromJob_Edit(t *testing.T) {
	svc, _, _, _, textUUID := setupMessageServiceTest(t)

	// Create a message first
	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "Original", "", "", nil)
	assertNoError(t, err)

	now := time.Now().UTC()
	payload, err := json.Marshal(map[string]interface{}{
		"action":       "edit",
		"message_uuid": dto.UUID,
		"content":      "Edited from job",
		"timestamp":    now,
	})
	assertNoError(t, err)

	err = svc.MutateFromJob(payload)
	assertNoError(t, err)

	msg, err := svc.msgRepo.GetByUUID(dto.UUID)
	assertNoError(t, err)
	if msg.Content != "Edited from job" {
		t.Errorf("expected 'Edited from job', got %s", msg.Content)
	}
}

func TestMessageService_MutateFromJob_Delete(t *testing.T) {
	svc, _, _, _, textUUID := setupMessageServiceTest(t)

	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "To be deleted", "", "", nil)
	assertNoError(t, err)

	payload, err := json.Marshal(map[string]interface{}{
		"action":       "delete",
		"message_uuid": dto.UUID,
	})
	assertNoError(t, err)

	err = svc.MutateFromJob(payload)
	assertNoError(t, err)

	// Message should be soft-deleted
	_, err = svc.msgRepo.GetByUUID(dto.UUID)
	if err == nil {
		t.Error("expected message to be soft-deleted")
	}
}

func TestMessageService_MutateFromJob_React(t *testing.T) {
	svc, _, _, _, textUUID := setupMessageServiceTest(t)

	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "Reactable", "", "", nil)
	assertNoError(t, err)

	payload, err := json.Marshal(map[string]interface{}{
		"action":       "react",
		"message_uuid": dto.UUID,
		"user_id":      "bob",
		"emoji":        "+1",
	})
	assertNoError(t, err)

	err = svc.MutateFromJob(payload)
	assertNoError(t, err)

	reactions, err := svc.msgRepo.ListReactions([]string{dto.UUID})
	assertNoError(t, err)
	if len(reactions) != 1 {
		t.Errorf("expected 1 reaction, got %d", len(reactions))
	}
}

func TestMessageService_MutateFromJob_UnknownAction(t *testing.T) {
	svc, _, _, _, _ := setupMessageServiceTest(t)

	payload, err := json.Marshal(map[string]interface{}{
		"action": "unknown_action",
	})
	assertNoError(t, err)

	err = svc.MutateFromJob(payload)
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestMessageService_MutateFromJob_InvalidPayload(t *testing.T) {
	svc, _, _, _, _ := setupMessageServiceTest(t)

	err := svc.MutateFromJob([]byte("invalid json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// ─── Search Tests ───

func TestMessageService_Search(t *testing.T) {
	svc, _, _, _, textUUID := setupMessageServiceTest(t)

	if _, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "alpha needle one", "", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Send(textUUID, testActorWithUUID("bob", "uuid-bob"), "plain message", "", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Send(textUUID, testActorWithUUID("carol", "uuid-carol"), "beta needle two", "", "", nil); err != nil {
		t.Fatal(err)
	}

	items, err := svc.Search(textUUID, testActorWithUUID("alice", "uuid-alice"), "needle", "")
	assertNoError(t, err)
	if len(items) != 2 {
		t.Fatalf("expected 2 search results, got %d", len(items))
	}
}

func TestMessageService_Search_RejectsVoiceRoom(t *testing.T) {
	svc, _, _, roomRepo, _ := setupMessageServiceTest(t)
	voiceUUID := mustFindVoiceUUID(roomRepo)
	_, err := svc.Search(voiceUUID, testActorWithUUID("alice", "uuid-alice"), "needle", "")
	assertErrorCode(t, err, pkg.FORBIDDEN)
}

// ─── broadcastRoomKey Tests ───

func TestBroadcastRoomKey_PlatformRoom(t *testing.T) {
	got := broadcastRoomKey("", "my-room")
	if got != "my-room" {
		t.Errorf("expected 'my-room', got %q", got)
	}
}

func TestBroadcastRoomKey_DomainRoom(t *testing.T) {
	got := broadcastRoomKey("dom-123", "my-room")
	if got != "dom-123:my-room" {
		t.Errorf("expected 'dom-123:my-room', got %q", got)
	}
}

// ─── Domain-scoped broadcast room key tests ───

func TestMessageService_Send_BroadcastUsesDomainScopedKey(t *testing.T) {
	svc, bus, _, roomRepo, _ := setupMessageServiceTest(t)
	svc.SetEventBus(bus)

	domainRoomUUID := addDomainTextRoom(roomRepo, "domain-text-room", "domain-abc")

	_, err := svc.Send(domainRoomUUID, testActorWithUUID("alice", "uuid-alice"), "hello", "", "", nil)
	assertNoError(t, err)

	if len(bus.rooms) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(bus.rooms))
	}
	if bus.rooms[0] != "domain-abc:domain-text-room" {
		t.Errorf("expected domain-scoped room key 'domain-abc:domain-text-room', got %q", bus.rooms[0])
	}
}

func TestMessageService_Send_BroadcastUsesPlainKeyForPlatformRoom(t *testing.T) {
	svc, bus, _, _, textUUID := setupMessageServiceTest(t)
	svc.SetEventBus(bus)

	_, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "hello", "", "", nil)
	assertNoError(t, err)

	if len(bus.rooms) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(bus.rooms))
	}
	// Platform room (no DomainUUID) should use plain room name
	if bus.rooms[0] != "test-text-room" {
		t.Errorf("expected plain room key 'test-text-room', got %q", bus.rooms[0])
	}
}

func TestMessageService_Edit_BroadcastUsesDomainScopedKey(t *testing.T) {
	svc, bus, _, roomRepo, _ := setupMessageServiceTest(t)
	svc.SetEventBus(bus)

	domainRoomUUID := addDomainTextRoom(roomRepo, "domain-room", "domain-xyz")

	dto, err := svc.Send(domainRoomUUID, testActorWithUUID("alice", "uuid-alice"), "orig", "", "", nil)
	assertNoError(t, err)

	// Reset to only track the Edit broadcast
	bus.reset()

	_, err = svc.Edit(domainRoomUUID, dto.UUID, testActorWithUUID("alice", "uuid-alice"), "edited")
	assertNoError(t, err)

	if len(bus.rooms) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(bus.rooms))
	}
	if bus.rooms[0] != "domain-xyz:domain-room" {
		t.Errorf("expected domain-scoped key, got %q", bus.rooms[0])
	}
}

func TestMessageService_Delete_BroadcastUsesDomainScopedKey(t *testing.T) {
	svc, bus, _, roomRepo, _ := setupMessageServiceTest(t)
	svc.SetEventBus(bus)

	domainRoomUUID := addDomainTextRoom(roomRepo, "domain-delete-room", "domain-del")
	dto, err := svc.Send(domainRoomUUID, testActorWithUUID("alice", "uuid-alice"), "orig", "", "", nil)
	assertNoError(t, err)

	bus.reset()
	assertNoError(t, svc.Delete(domainRoomUUID, dto.UUID, testActorWithUUID("alice", "uuid-alice"), false))

	if len(bus.calls) != 1 || bus.calls[0] != "message:deleted" {
		t.Fatalf("expected message:deleted broadcast, got %v", bus.calls)
	}
	if bus.rooms[0] != "domain-del:domain-delete-room" {
		t.Errorf("expected domain-scoped key, got %q", bus.rooms[0])
	}
}

func TestMessageService_React_BroadcastUsesDomainScopedKey(t *testing.T) {
	svc, bus, _, roomRepo, _ := setupMessageServiceTest(t)
	svc.SetEventBus(bus)

	domainRoomUUID := addDomainTextRoom(roomRepo, "domain-react-room", "domain-react")
	dto, err := svc.Send(domainRoomUUID, testActorWithUUID("alice", "uuid-alice"), "orig", "", "", nil)
	assertNoError(t, err)

	bus.reset()
	assertNoError(t, svc.React(domainRoomUUID, dto.UUID, testActorWithUUID("alice", "uuid-alice"), "+1"))

	if len(bus.calls) != 1 || bus.calls[0] != "message:reaction" {
		t.Fatalf("expected message:reaction broadcast, got %v", bus.calls)
	}
	if bus.rooms[0] != "domain-react:domain-react-room" {
		t.Errorf("expected domain-scoped key, got %q", bus.rooms[0])
	}
}

func TestMessageService_Unreact_BroadcastUsesDomainScopedKey(t *testing.T) {
	svc, bus, _, roomRepo, _ := setupMessageServiceTest(t)
	svc.SetEventBus(bus)

	domainRoomUUID := addDomainTextRoom(roomRepo, "domain-unreact-room", "domain-unreact")
	dto, err := svc.Send(domainRoomUUID, testActorWithUUID("alice", "uuid-alice"), "orig", "", "", nil)
	assertNoError(t, err)
	assertNoError(t, svc.React(domainRoomUUID, dto.UUID, testActorWithUUID("alice", "uuid-alice"), "+1"))

	bus.reset()
	assertNoError(t, svc.Unreact(domainRoomUUID, dto.UUID, testActorWithUUID("alice", "uuid-alice"), "+1"))

	if len(bus.calls) != 1 || bus.calls[0] != "message:reaction" {
		t.Fatalf("expected message:reaction broadcast, got %v", bus.calls)
	}
	if bus.rooms[0] != "domain-unreact:domain-unreact-room" {
		t.Errorf("expected domain-scoped key, got %q", bus.rooms[0])
	}
}

// ─── Durability & Compensation Tests ───

func TestMessageService_Send_BroadcastFailurePersistsSync(t *testing.T) {
	svc, bus, queue, _, textUUID := setupMessageServiceTest(t)
	t.Cleanup(func() { bus.reset(); queue.reset() })

	svc.SetEventBus(bus)
	svc.SetJobQueue(queue)
	bus.err = errors.New("bus down")

	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "durable message", "", "", nil)
	assertNoError(t, err)

	msg, err := svc.msgRepo.GetByUUID(dto.UUID)
	assertNoError(t, err)
	if msg.Content != "durable message" {
		t.Fatalf("expected sync-persisted message, got %q", msg.Content)
	}
	if len(queue.jobs) != 0 {
		t.Fatalf("expected no async job after broadcast failure, got %d", len(queue.jobs))
	}
}

func TestMessageService_PersistFromJob_Idempotent(t *testing.T) {
	svc, _, _, _, textUUID := setupMessageServiceTest(t)

	dto, err := svc.Send(textUUID, testActorWithUUID("alice", "uuid-alice"), "already persisted", "", "", nil)
	assertNoError(t, err)

	payload, err := json.Marshal(map[string]interface{}{
		"uuid":        dto.UUID,
		"room_uuid":   textUUID,
		"author_id":   "alice",
		"author_uuid": "uuid-alice",
		"content":     "already persisted",
		"mentions":    []string{},
		"created_at":  dto.CreatedAt,
	})
	assertNoError(t, err)
	assertNoError(t, svc.PersistFromJob(payload))

	msg, err := svc.msgRepo.GetByUUID(dto.UUID)
	assertNoError(t, err)
	if msg.Content != "already persisted" {
		t.Fatalf("unexpected content %q", msg.Content)
	}
}

func TestMessageService_MutateFromJob_EditUpsertsMissingMessage(t *testing.T) {
	svc, _, _, _, textUUID := setupMessageServiceTest(t)

	msgUUID := uuid.New().String()
	now := time.Now().UTC()
	payload, err := json.Marshal(map[string]interface{}{
		"action":       "edit",
		"message_uuid": msgUUID,
		"room_uuid":    textUUID,
		"author_id":    "alice",
		"author_uuid":  "uuid-alice",
		"content":      "recovered edit",
		"timestamp":    now,
	})
	assertNoError(t, err)
	assertNoError(t, svc.MutateFromJob(payload))

	msg, err := svc.msgRepo.GetByUUID(msgUUID)
	assertNoError(t, err)
	if msg.Content != "recovered edit" {
		t.Fatalf("expected upserted edit content, got %q", msg.Content)
	}
}

func TestMessageService_MutateFromJob_DeleteUpsertsMissingMessage(t *testing.T) {
	svc, _, _, _, textUUID := setupMessageServiceTest(t)

	msgUUID := uuid.New().String()
	payload, err := json.Marshal(map[string]interface{}{
		"action":       "delete",
		"message_uuid": msgUUID,
		"room_uuid":    textUUID,
		"author_id":    "alice",
		"author_uuid":  "uuid-alice",
		"timestamp":    time.Now().UTC(),
	})
	assertNoError(t, err)
	assertNoError(t, svc.MutateFromJob(payload))

	if _, err := svc.msgRepo.GetByUUID(msgUUID); err == nil {
		t.Fatal("expected upserted message to be soft-deleted")
	}
}

func TestMessageService_ListHistory_PlatformPasswordRoomRejectsWrongPassword(t *testing.T) {
	svc, _, _, roomRepo, _ := setupMessageServiceTest(t)
	roomUUID := uuid.New().String()
	hashed, _ := pkg.HashPassword("secret")
	roomRepo.mu.Lock()
	roomRepo.rooms[roomUUID] = &model.Room{
		UUID:       roomUUID,
		Name:       "private-text",
		Type:       model.RoomTypeText,
		DomainUUID: "",
		Password:   hashed,
	}
	roomRepo.mu.Unlock()

	actor := MessageActor{Identity: "bob", UserUUID: "uuid-bob"}
	_, _, _, err := svc.ListHistory(roomUUID, actor, "", 100, "wrong")
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) || appErr.Code != pkg.FORBIDDEN {
		t.Fatalf("want FORBIDDEN, got %v", err)
	}
}

func TestMessageService_ListHistory_PlatformPasswordRoomAllowsCorrectPassword(t *testing.T) {
	svc, _, _, roomRepo, _ := setupMessageServiceTest(t)
	roomUUID := uuid.New().String()
	hashed, _ := pkg.HashPassword("secret")
	roomRepo.mu.Lock()
	roomRepo.rooms[roomUUID] = &model.Room{
		UUID:       roomUUID,
		Name:       "private-text",
		Type:       model.RoomTypeText,
		DomainUUID: "",
		Password:   hashed,
	}
	roomRepo.mu.Unlock()

	actor := MessageActor{Identity: "bob", UserUUID: "uuid-bob"}
	if _, _, _, err := svc.ListHistory(roomUUID, actor, "", 100, "secret"); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
}
