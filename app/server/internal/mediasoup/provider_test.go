package mediasoup

import (
	"errors"
	"sync"
	"testing"

	"GOSpeak/internal/pkg"
)

type stubProviderBridge struct {
	mu              sync.Mutex
	listedRoom      string
	closedIdentity  string
	pausedProducer  string
	resumedProducer string
	pausedIdentity  string
	resumedIdentity string
	listResult      []ParticipantInfo
	closeResult     []string
}

func (s *stubProviderBridge) ListParticipants(roomID string) ([]ParticipantInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listedRoom = roomID
	return s.listResult, nil
}

func (s *stubProviderBridge) CloseParticipant(roomID, identity string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closedIdentity = identity
	return s.closeResult, nil
}

func (s *stubProviderBridge) PauseProducer(roomID, producerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pausedProducer = producerID
	return nil
}

func (s *stubProviderBridge) ResumeProducer(roomID, producerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resumedProducer = producerID
	return nil
}

func (s *stubProviderBridge) PauseParticipant(roomID, identity string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pausedIdentity = identity
	return nil
}

func (s *stubProviderBridge) ResumeParticipant(roomID, identity string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resumedIdentity = identity
	return nil
}

func TestListParticipants_Delegates(t *testing.T) {
	svc := &Service{Bridge: nil}
	svc.partBridge = &stubProviderBridge{listResult: []ParticipantInfo{{Identity: "alice"}}}
	got, err := svc.ListParticipants("r1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 || got[0].Identity != "alice" {
		t.Fatalf("got %v", got)
	}
}

func TestMuteParticipant_ByTrackSid(t *testing.T) {
	stub := &stubProviderBridge{}
	svc := &Service{partBridge: stub}
	if err := svc.MuteParticipant("r1", "alice", "p1", true); err != nil {
		t.Fatalf("err: %v", err)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if stub.pausedProducer != "p1" {
		t.Fatalf("expected PauseProducer p1, got %q", stub.pausedProducer)
	}
}

func TestMuteParticipant_ByIdentity(t *testing.T) {
	stub := &stubProviderBridge{}
	svc := &Service{partBridge: stub}
	if err := svc.MuteParticipant("r1", "alice", "", false); err != nil {
		t.Fatalf("err: %v", err)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if stub.resumedIdentity != "alice" {
		t.Fatalf("expected ResumeParticipant alice, got %q", stub.resumedIdentity)
	}
}

func TestRemoveParticipant_Delegates(t *testing.T) {
	stub := &stubProviderBridge{closeResult: []string{"p1"}}
	svc := &Service{partBridge: stub}
	if err := svc.RemoveParticipant("r1", "alice"); err != nil {
		t.Fatalf("err: %v", err)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if stub.closedIdentity != "alice" {
		t.Fatalf("expected CloseParticipant alice, got %q", stub.closedIdentity)
	}
}

func TestGenerateAdminToken_NotSupported(t *testing.T) {
	svc := &Service{Bridge: NewBridgeClient("http://localhost")}
	if _, err := svc.GenerateAdminToken(); !errors.Is(err, pkg.ErrSFUNotSupported) {
		t.Fatalf("GenerateAdminToken: want ErrSFUNotSupported, got %v", err)
	}
}
