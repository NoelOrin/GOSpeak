package service

import (
	"errors"
	"testing"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
)

type fakeSFUProvider struct {
	generatedRoom     string
	generatedIdentity string
	streamRoom        string
}

func (f *fakeSFUProvider) ProviderName() string { return "fake" }
func (f *fakeSFUProvider) Capabilities() sfu.Capabilities {
	return sfu.Capabilities{}
}
func (f *fakeSFUProvider) GenerateToken(room, identity string) (string, error) {
	f.generatedRoom = room
	f.generatedIdentity = identity
	return "token-" + room, nil
}
func (f *fakeSFUProvider) GenerateAdminToken() (string, error) {
	return "", pkg.NewErrSFUNotSupported()
}
func (f *fakeSFUProvider) ListRooms() ([]sfu.RoomSummary, error) {
	return nil, nil
}
func (f *fakeSFUProvider) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	return nil, nil
}
func (f *fakeSFUProvider) MuteParticipant(room, identity, trackSid string, muted bool) error {
	return nil
}
func (f *fakeSFUProvider) RemoveParticipant(room, identity string) error {
	return nil
}
func (f *fakeSFUProvider) DeleteRoom(room string) error {
	return nil
}
func (f *fakeSFUProvider) GetHost() string { return "wss://fake" }
func (f *fakeSFUProvider) StreamName(room, identity string) string {
	return "stream-" + room
}
func (f *fakeSFUProvider) StreamInfo(room, identity string) (string, string, error) {
	f.streamRoom = room
	return "stream-" + room, "stream-token", nil
}

type ownerAwareFakeProvider struct {
	fakeSFUProvider
	ownerUUID string
}

func (p *ownerAwareFakeProvider) GenerateTokenForUser(room, identity, ownerUUID string) (string, error) {
	p.ownerUUID = ownerUUID
	return "token-" + room, nil
}

func TestGetJoinToken_ForwardsOwnerToOwnerAwareProvider(t *testing.T) {
	p := &ownerAwareFakeProvider{}
	svc := NewSFUService(p, nil)

	if _, err := svc.GetJoinToken("", "lobby", "user-1", "uuid-user-1", ""); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if p.ownerUUID != "uuid-user-1" {
		t.Fatalf("ownerUUID = %q, want uuid-user-1", p.ownerUUID)
	}
}

func assertForbidden(t *testing.T, err error) {
	t.Helper()
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *pkg.AppError, got %v", err)
	}
	if appErr.Code != pkg.FORBIDDEN {
		t.Fatalf("expected FORBIDDEN (%d), got %d", pkg.FORBIDDEN, appErr.Code)
	}
}

func TestGetJoinToken_NonDomainMemberForbidden(t *testing.T) {
	svc := NewSFUService(&fakeSFUProvider{}, nil)
	svc.SetDomainMemberChecker(func(domainUUID, userUUID string) bool { return false })

	_, err := svc.GetJoinToken("domain-a", "lobby", "user-1", "user-1", "")
	assertForbidden(t, err)
}

func TestGetJoinToken_NilDomainCheckerForbidden(t *testing.T) {
	svc := NewSFUService(&fakeSFUProvider{}, nil)

	_, err := svc.GetJoinToken("domain-a", "lobby", "user-1", "user-1", "")
	assertForbidden(t, err)
}

func TestGetJoinToken_DomainMemberUsesCompositeRoom(t *testing.T) {
	p := &fakeSFUProvider{}
	svc := NewSFUService(p, nil)
	svc.SetDomainMemberChecker(func(domainUUID, userUUID string) bool {
		return domainUUID == "domain-a" && userUUID == "user-1"
	})

	res, err := svc.GetJoinToken("domain-a", "lobby", "user-1", "user-1", "")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if p.generatedRoom != "domain-a:lobby" {
		t.Fatalf("GenerateToken room = %q, want %q", p.generatedRoom, "domain-a:lobby")
	}
	if p.streamRoom != "domain-a:lobby" {
		t.Fatalf("StreamInfo room = %q, want %q", p.streamRoom, "domain-a:lobby")
	}
	if res.SFURoom != "domain-a:lobby" {
		t.Fatalf("SFURoom = %q, want %q", res.SFURoom, "domain-a:lobby")
	}
}

func TestGetJoinToken_PlatformRoomKeepsLogicalName(t *testing.T) {
	p := &fakeSFUProvider{}
	svc := NewSFUService(p, nil)

	res, err := svc.GetJoinToken("", "lobby", "user-1", "user-1", "")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if p.generatedRoom != "lobby" {
		t.Fatalf("GenerateToken room = %q, want %q", p.generatedRoom, "lobby")
	}
	if p.streamRoom != "lobby" {
		t.Fatalf("StreamInfo room = %q, want %q", p.streamRoom, "lobby")
	}
	if res.SFURoom != "lobby" {
		t.Fatalf("SFURoom = %q, want %q", res.SFURoom, "lobby")
	}
}
