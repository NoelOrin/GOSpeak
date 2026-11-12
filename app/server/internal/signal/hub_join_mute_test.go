package signal

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
)

// recordingMuteProvider 记录 unmute 媒体调用，验证 join 自愈清理。
type recordingMuteProvider struct {
	capsProvider
	mu      sync.Mutex
	unmuted []string // room\x00identity
	muted   []string // room\x00identity
	ttlSeen int
}

func (p *recordingMuteProvider) MuteParticipantTimed(room, identity, trackSid string, muted bool, ttlSeconds int) error {
	p.mu.Lock()
	if muted {
		p.muted = append(p.muted, room+"\x00"+identity)
		p.ttlSeen = ttlSeconds
	} else {
		p.unmuted = append(p.unmuted, room+"\x00"+identity)
	}
	p.mu.Unlock()
	return p.capsProvider.MuteParticipantTimed(room, identity, trackSid, muted, ttlSeconds)
}

func (p *recordingMuteProvider) unmuteTargets() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.unmuted...)
}

func (p *recordingMuteProvider) muteTargets() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.muted...)
}

func (p *recordingMuteProvider) lastTTL() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ttlSeen
}

// plainMuteProvider 只实现 MuteParticipant，不实现 TimedMuteProvider，覆盖非 Timed 回退分支。
type plainMuteProvider struct {
	caps    sfu.Capabilities
	mu      sync.Mutex
	unmuted []string // room\x00identity
	muted   []string // room\x00identity
	err     error
}

func (p *plainMuteProvider) ProviderName() string           { return "plain" }
func (p *plainMuteProvider) Capabilities() sfu.Capabilities { return p.caps }
func (p *plainMuteProvider) GenerateToken(string, string) (string, error) {
	return "t", nil
}
func (p *plainMuteProvider) ListRooms() ([]sfu.RoomSummary, error) { return nil, nil }
func (p *plainMuteProvider) ListParticipants(string) ([]sfu.ParticipantSummary, error) {
	return nil, nil
}
func (p *plainMuteProvider) MuteParticipant(room, identity, trackSid string, muted bool) error {
	p.mu.Lock()
	if muted {
		p.muted = append(p.muted, room+"\x00"+identity)
	} else {
		p.unmuted = append(p.unmuted, room+"\x00"+identity)
	}
	err := p.err
	p.mu.Unlock()
	return err
}
func (p *plainMuteProvider) RemoveParticipant(room, identity string) error {
	return nil
}
func (p *plainMuteProvider) DeleteRoom(string) error { return nil }
func (p *plainMuteProvider) GetHost() string         { return "host" }

func (p *plainMuteProvider) unmuteTargets() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.unmuted...)
}

func (p *plainMuteProvider) muteTargets() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.muted...)
}

// flipMuteStore 首次查询返回未禁言，后续查询按配置返回，用于模拟 join 自愈窗口内新禁言。
type flipMuteStore struct {
	mu           sync.Mutex
	calls        int
	secondResult bool
	secondMute   *model.Mute
	secondErr    error
}

func (s *flipMuteStore) IsMutedByIdentity(identity string) (bool, *model.Mute, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.calls == 1 {
		return false, nil, nil
	}
	return s.secondResult, s.secondMute, s.secondErr
}

func (s *flipMuteStore) IsMutedBatch(identities []string) (map[string]bool, error) {
	return make(map[string]bool, len(identities)), nil
}

func newResidueJoinHub(prov *recordingMuteProvider, mStore muteStore) *Hub {
	return newResidueJoinHubWithStore(prov, mStore, nil)
}

func newResidueJoinHubWithStore(prov *recordingMuteProvider, mStore muteStore, store roomStore) *Hub {
	hub := NewHub(store, mStore, &idUserStore{users: map[uint]*model.User{
		1: {ID: 1, Name: "alice"},
	}}, nil)
	hub.fanout = newMockBroadcaster()
	hub.sfuProvider = prov
	hub.SetDomainChecker(func(domainUUID, userUUID string) bool { return true })
	return hub
}

type alwaysMutedStore struct{}

func (alwaysMutedStore) IsMutedByIdentity(identity string) (bool, *model.Mute, error) {
	return true, &model.Mute{}, nil
}

func (alwaysMutedStore) IsMutedBatch(identities []string) (map[string]bool, error) {
	out := make(map[string]bool, len(identities))
	for _, identity := range identities {
		out[identity] = true
	}
	return out, nil
}

func TestOnRoomJoin_ClearsMuteResidueForUnmutedUser(t *testing.T) {
	prov := &recordingMuteProvider{capsProvider: capsProvider{caps: sfu.Capabilities{
		ServerMute: true,
		MuteLevel:  sfu.EnforcementDegraded,
	}}}
	hub := newResidueJoinHub(prov, nil)

	conn := fakeConn("alice")
	if _, err := hub.OnRoomJoin(conn, `{"room":"lobby","domain_uuid":"dom-a","identity":"alice"}`); err != nil {
		t.Fatalf("join should succeed for unmuted user: %v", err)
	}

	wantRoom := "dom-a:lobby"
	targets := prov.unmuteTargets()
	if len(targets) != 1 {
		t.Fatalf("expected 1 residue-clearing unmute, got %v", targets)
	}
	if targets[0] != wantRoom+"\x00alice" {
		t.Fatalf("unmute target = %q, want %q", targets[0], wantRoom+"\x00alice")
	}
	if !hub.fanout.(*mockBroadcaster).didJoin(conn.ID(), wantRoom) {
		t.Fatal("expected client to reach fanout join after residue clear")
	}
}

func TestOnRoomJoin_MutedUserRejectedWithoutResidueClear(t *testing.T) {
	prov := &recordingMuteProvider{capsProvider: capsProvider{caps: sfu.Capabilities{
		ServerMute: true,
		MuteLevel:  sfu.EnforcementDegraded,
	}}}
	hub := newResidueJoinHub(prov, alwaysMutedStore{})

	ack, err := hub.OnRoomJoin(fakeConn("alice"), `{"room":"lobby","domain_uuid":"dom-a","identity":"alice"}`)
	if err != nil {
		t.Fatalf("muted join should be rejected via ack, got error: %v", err)
	}
	if !strings.Contains(ack, "user is muted") {
		t.Fatalf("expected muted rejection ack, got %q", ack)
	}
	if targets := prov.unmuteTargets(); len(targets) != 0 {
		t.Fatalf("muted join must not clear residue, got %v", targets)
	}
}

func TestOnRoomJoin_TextRoomSkipsMuteResidueClear(t *testing.T) {
	prov := &recordingMuteProvider{capsProvider: capsProvider{caps: sfu.Capabilities{
		ServerMute: true,
		MuteLevel:  sfu.EnforcementDegraded,
	}}}
	store := &mockRoomStore{rooms: []model.Room{{
		Name:       "text-chat",
		DomainUUID: "dom-a",
		Type:       model.RoomTypeText,
	}}}
	hub := newResidueJoinHubWithStore(prov, nil, store)

	conn := fakeConn("alice")
	if _, err := hub.OnRoomJoin(conn, `{"room":"text-chat","domain_uuid":"dom-a","identity":"alice"}`); err != nil {
		t.Fatalf("text room join should succeed: %v", err)
	}
	if targets := prov.unmuteTargets(); len(targets) != 0 {
		t.Fatalf("text room must not clear media mute residue, got %v", targets)
	}
	if !hub.fanout.(*mockBroadcaster).didJoin(conn.ID(), "dom-a:text-chat") {
		t.Fatal("expected text room client to reach fanout join")
	}
}

func TestOnRoomJoin_NoServerMuteCapabilitySkipsResidueClear(t *testing.T) {
	prov := &recordingMuteProvider{capsProvider: capsProvider{caps: sfu.Capabilities{
		ServerMute: false,
		MuteLevel:  sfu.EnforcementSoft,
	}}}
	hub := newResidueJoinHub(prov, nil)

	if _, err := hub.OnRoomJoin(fakeConn("alice"), `{"room":"lobby","domain_uuid":"dom-a","identity":"alice"}`); err != nil {
		t.Fatalf("join should succeed without server mute capability: %v", err)
	}
	if targets := prov.unmuteTargets(); len(targets) != 0 {
		t.Fatalf("provider without ServerMute must not clear residue, got %v", targets)
	}
}

func TestOnRoomJoin_HardProviderSkipsResidueClear(t *testing.T) {
	prov := &recordingMuteProvider{capsProvider: capsProvider{caps: sfu.Capabilities{
		ServerMute: true,
		MuteLevel:  sfu.EnforcementHard,
	}}}
	hub := newResidueJoinHub(prov, nil)

	conn := fakeConn("alice")
	if _, err := hub.OnRoomJoin(conn, `{"room":"lobby","domain_uuid":"dom-a","identity":"alice"}`); err != nil {
		t.Fatalf("join should succeed on hard provider: %v", err)
	}
	if targets := prov.unmuteTargets(); len(targets) != 0 {
		t.Fatalf("hard track-level mute must not clear residue, got %v", targets)
	}
	if !hub.fanout.(*mockBroadcaster).didJoin(conn.ID(), "dom-a:lobby") {
		t.Fatal("expected client to reach fanout join on hard provider")
	}
}

func TestOnRoomJoin_MediaMuteLevelGatesResidueClear(t *testing.T) {
	prov := &recordingMuteProvider{capsProvider: capsProvider{caps: sfu.Capabilities{
		ServerMute: false,
		MuteLevel:  sfu.EnforcementDegraded,
	}}}
	hub := newResidueJoinHub(prov, nil)

	conn := fakeConn("alice")
	if _, err := hub.OnRoomJoin(conn, `{"room":"lobby","domain_uuid":"dom-a","identity":"alice"}`); err != nil {
		t.Fatalf("join should succeed for media-enabled provider: %v", err)
	}
	if targets := prov.unmuteTargets(); len(targets) != 1 {
		t.Fatalf("MuteLevel should gate residue clear regardless of ServerMute, got %v", targets)
	}
}

func TestOnRoomJoin_ClearsMuteResidue_NonTimedProviderFallback(t *testing.T) {
	prov := &plainMuteProvider{caps: sfu.Capabilities{MuteLevel: sfu.EnforcementDegraded}}
	hub := NewHub(nil, nil, &idUserStore{users: map[uint]*model.User{
		1: {ID: 1, Name: "alice"},
	}}, nil)
	hub.fanout = newMockBroadcaster()
	hub.sfuProvider = prov
	hub.SetDomainChecker(func(domainUUID, userUUID string) bool { return true })

	conn := fakeConn("alice")
	if _, err := hub.OnRoomJoin(conn, `{"room":"lobby","domain_uuid":"dom-a","identity":"alice"}`); err != nil {
		t.Fatalf("join should succeed for non-timed provider: %v", err)
	}
	targets := prov.unmuteTargets()
	if len(targets) != 1 || targets[0] != "dom-a:lobby\x00alice" {
		t.Fatalf("expected non-timed unmute fallback, got %v", targets)
	}
	if !hub.fanout.(*mockBroadcaster).didJoin(conn.ID(), "dom-a:lobby") {
		t.Fatal("expected client to reach fanout join after non-timed residue clear")
	}
}

func TestOnRoomJoin_NeverMutedUserClearsResidueIdempotently(t *testing.T) {
	prov := &recordingMuteProvider{capsProvider: capsProvider{caps: sfu.Capabilities{
		ServerMute: true,
		MuteLevel:  sfu.EnforcementDegraded,
	}}}
	hub := newResidueJoinHub(prov, newMockMuteStore())

	conn := fakeConn("alice")
	if _, err := hub.OnRoomJoin(conn, `{"room":"lobby","domain_uuid":"dom-a","identity":"alice"}`); err != nil {
		t.Fatalf("join should succeed for never-muted user: %v", err)
	}
	targets := prov.unmuteTargets()
	if len(targets) != 1 || targets[0] != "dom-a:lobby\x00alice" {
		t.Fatalf("expected exactly one idempotent residue-clearing unmute, got %v", targets)
	}
	if targets := prov.muteTargets(); len(targets) != 0 {
		t.Fatalf("never-muted user must not be re-muted, got %v", targets)
	}
	if !hub.fanout.(*mockBroadcaster).didJoin(conn.ID(), "dom-a:lobby") {
		t.Fatal("expected client to reach fanout join for never-muted user")
	}
}

func TestOnRoomJoin_NewMuteDuringResidueClearRemutes(t *testing.T) {
	expires := time.Now().Add(60 * time.Second)
	store := &flipMuteStore{
		secondResult: true,
		secondMute:   &model.Mute{Permanent: false, ExpiresAt: &expires},
	}
	prov := &recordingMuteProvider{capsProvider: capsProvider{caps: sfu.Capabilities{
		ServerMute: true,
		MuteLevel:  sfu.EnforcementDegraded,
	}}}
	hub := newResidueJoinHub(prov, store)

	conn := fakeConn("alice")
	ack, err := hub.OnRoomJoin(conn, `{"room":"lobby","domain_uuid":"dom-a","identity":"alice"}`)
	if err != nil {
		t.Fatalf("muted recheck should be returned via ack, got error: %v", err)
	}
	if !strings.Contains(ack, "user is muted") {
		t.Fatalf("expected muted rejection ack after recheck, got %q", ack)
	}
	if targets := prov.unmuteTargets(); len(targets) != 1 || targets[0] != "dom-a:lobby\x00alice" {
		t.Fatalf("expected residue-clearing unmute before recheck, got %v", targets)
	}
	if targets := prov.muteTargets(); len(targets) != 1 || targets[0] != "dom-a:lobby\x00alice" {
		t.Fatalf("expected re-mute for new mute, got %v", targets)
	}
	if ttl := prov.lastTTL(); ttl < 50 || ttl > 60 {
		t.Fatalf("expected remaining TTL near 60s, got %d", ttl)
	}
	if hub.fanout.(*mockBroadcaster).didJoin(conn.ID(), "dom-a:lobby") {
		t.Fatal("muted recheck must not reach fanout join")
	}
	fanout := hub.fanout.(*mockBroadcaster)
	if len(fanout.broadcasts[EventMemberMuted]) != 0 {
		t.Fatal("recheck must not broadcast member:muted; admin mute already broadcast")
	}
}

func TestOnRoomJoin_NewMuteDuringResidueClearRemainingZeroSkipsRemute(t *testing.T) {
	cases := []struct {
		name    string
		expires time.Time
	}{
		{"expired", time.Now().Add(-5 * time.Second)},
		{"less than one second", time.Now().Add(500 * time.Millisecond)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &flipMuteStore{
				secondResult: true,
				secondMute:   &model.Mute{Permanent: false, ExpiresAt: &tc.expires},
			}
			prov := &recordingMuteProvider{capsProvider: capsProvider{caps: sfu.Capabilities{
				ServerMute: true,
				MuteLevel:  sfu.EnforcementDegraded,
			}}}
			hub := newResidueJoinHub(prov, store)

			conn := fakeConn("alice")
			ack, err := hub.OnRoomJoin(conn, `{"room":"lobby","domain_uuid":"dom-a","identity":"alice"}`)
			if err != nil {
				t.Fatalf("muted recheck should be returned via ack, got error: %v", err)
			}
			if !strings.Contains(ack, "user is muted") {
				t.Fatalf("expected muted rejection ack after remaining-zero recheck, got %q", ack)
			}
			if targets := prov.unmuteTargets(); len(targets) != 1 {
				t.Fatalf("expected residue-clearing unmute before recheck, got %v", targets)
			}
			if targets := prov.muteTargets(); len(targets) != 0 {
				t.Fatalf("remaining<=0 temporary mute must not be re-muted, got %v", targets)
			}
			if hub.fanout.(*mockBroadcaster).didJoin(conn.ID(), "dom-a:lobby") {
				t.Fatal("muted recheck must not reach fanout join")
			}
		})
	}
}

func TestOnRoomJoin_NewMuteDuringResidueClearRemutes_NonTimed(t *testing.T) {
	expires := time.Now().Add(30 * time.Second)
	store := &flipMuteStore{
		secondResult: true,
		secondMute:   &model.Mute{Permanent: false, ExpiresAt: &expires},
	}
	prov := &plainMuteProvider{caps: sfu.Capabilities{MuteLevel: sfu.EnforcementDegraded}}
	hub := NewHub(nil, store, &idUserStore{users: map[uint]*model.User{
		1: {ID: 1, Name: "alice"},
	}}, nil)
	hub.fanout = newMockBroadcaster()
	hub.sfuProvider = prov
	hub.SetDomainChecker(func(domainUUID, userUUID string) bool { return true })

	conn := fakeConn("alice")
	ack, err := hub.OnRoomJoin(conn, `{"room":"lobby","domain_uuid":"dom-a","identity":"alice"}`)
	if err != nil {
		t.Fatalf("muted recheck should be returned via ack, got error: %v", err)
	}
	if !strings.Contains(ack, "user is muted") {
		t.Fatalf("expected muted rejection ack after non-timed recheck, got %q", ack)
	}
	if targets := prov.unmuteTargets(); len(targets) != 1 {
		t.Fatalf("expected non-timed residue-clearing unmute, got %v", targets)
	}
	if targets := prov.muteTargets(); len(targets) != 1 || targets[0] != "dom-a:lobby\x00alice" {
		t.Fatalf("expected non-timed re-mute for new mute, got %v", targets)
	}
	if hub.fanout.(*mockBroadcaster).didJoin(conn.ID(), "dom-a:lobby") {
		t.Fatal("muted recheck must not reach fanout join")
	}
}

func TestOnRoomJoin_ResidueRecheckErrorKeepsUnmute(t *testing.T) {
	store := &flipMuteStore{secondErr: errors.New("db down")}
	prov := &recordingMuteProvider{capsProvider: capsProvider{caps: sfu.Capabilities{
		ServerMute: true,
		MuteLevel:  sfu.EnforcementDegraded,
	}}}
	hub := newResidueJoinHub(prov, store)

	conn := fakeConn("alice")
	if _, err := hub.OnRoomJoin(conn, `{"room":"lobby","domain_uuid":"dom-a","identity":"alice"}`); err != nil {
		t.Fatalf("recheck failure must not block join: %v", err)
	}
	if targets := prov.unmuteTargets(); len(targets) != 1 {
		t.Fatalf("recheck failure must keep unmute result, got %v", targets)
	}
	if targets := prov.muteTargets(); len(targets) != 0 {
		t.Fatalf("recheck failure must not re-mute, got %v", targets)
	}
	if !hub.fanout.(*mockBroadcaster).didJoin(conn.ID(), "dom-a:lobby") {
		t.Fatal("expected client to reach fanout join when recheck fails")
	}
}

func TestOnRoomJoin_CleanupFailureDoesNotBlockJoin(t *testing.T) {
	prov := &recordingMuteProvider{capsProvider: capsProvider{
		caps: sfu.Capabilities{
			ServerMute: true,
			MuteLevel:  sfu.EnforcementDegraded,
		},
		muteErr: errors.New("sfu down"),
	}}
	hub := newResidueJoinHub(prov, nil)

	conn := fakeConn("alice")
	if _, err := hub.OnRoomJoin(conn, `{"room":"lobby","domain_uuid":"dom-a","identity":"alice"}`); err != nil {
		t.Fatalf("join must not be blocked by best-effort residue clear: %v", err)
	}
	if targets := prov.unmuteTargets(); len(targets) != 1 {
		t.Fatalf("expected residue clear attempt despite failure, got %v", targets)
	}
	if !hub.fanout.(*mockBroadcaster).didJoin(conn.ID(), "dom-a:lobby") {
		t.Fatal("expected client to reach fanout join despite residue clear failure")
	}
}

func TestIsExpectedMuteResidueError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"sfu not supported", pkg.ErrSFUNotSupported, true},
		{"app error not found", pkg.NewAppError(pkg.NOT_FOUND, "stream not found"), true},
		{"participant not found message", errors.New("participant not found"), true},
		{"plain error", errors.New("upstream down"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isExpectedMuteResidueError(tc.err); got != tc.want {
				t.Fatalf("isExpectedMuteResidueError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
