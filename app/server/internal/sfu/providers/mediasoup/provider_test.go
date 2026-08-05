package mediasoup

import (
	"errors"
	"net/http"
	"reflect"
	"sync"
	"testing"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
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
	muteErr         error
	resumeErr       error
	closeErr        error
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
	return s.closeResult, s.closeErr
}

func (s *stubProviderBridge) PauseProducer(roomID, producerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pausedProducer = producerID
	return s.muteErr
}

func (s *stubProviderBridge) ResumeProducer(roomID, producerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resumedProducer = producerID
	return s.resumeErr
}

func (s *stubProviderBridge) PauseParticipant(roomID, identity string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pausedIdentity = identity
	return s.muteErr
}

func (s *stubProviderBridge) ResumeParticipant(roomID, identity string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resumedIdentity = identity
	return s.resumeErr
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

func TestCapabilities(t *testing.T) {
	caps := (&Service{}).Capabilities()
	if !reflect.DeepEqual(caps, sfu.CapabilitiesFor("mediasoup")) {
		t.Fatalf("Capabilities() = %+v, want %+v", caps, sfu.CapabilitiesFor("mediasoup"))
	}
}

func assertMediasoupAppErrorCode(t *testing.T, err error, want pkg.ErrCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %d, got nil", want)
	}
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *pkg.AppError, got %T: %v", err, err)
	}
	if appErr.Code != want {
		t.Fatalf("error code = %d, want %d: %v", appErr.Code, want, err)
	}
}

func TestMuteParticipant_Error(t *testing.T) {
	svc := &Service{partBridge: &stubProviderBridge{muteErr: errors.New("bridge down")}}
	assertMediasoupAppErrorCode(t, svc.MuteParticipant("r1", "alice", "", true), pkg.SFU_ERROR)
}

func TestRemoveParticipant_Error(t *testing.T) {
	svc := &Service{partBridge: &stubProviderBridge{closeErr: errors.New("bridge down")}}
	assertMediasoupAppErrorCode(t, svc.RemoveParticipant("r1", "alice"), pkg.SFU_ERROR)
}

func TestRemoveParticipant_NotFoundIsNoop(t *testing.T) {
	svc := &Service{partBridge: &stubProviderBridge{closeErr: ErrParticipantNotFound}}
	if err := svc.RemoveParticipant("r1", "alice"); err != nil {
		t.Fatalf("RemoveParticipant not-found: %v", err)
	}
}

func TestDeleteRoom_Error(t *testing.T) {
	b := newMockWorker(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && r.URL.Path == "/rooms/r1" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	})
	svc := &Service{Bridge: b, partBridge: b}
	assertMediasoupAppErrorCode(t, svc.DeleteRoom("r1"), pkg.SFU_ERROR)
}
