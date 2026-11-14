package factory

import (
	"errors"
	"testing"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
)

type stubProvider struct {
	name string
}

func (s *stubProvider) ProviderName() string { return s.name }
func (s *stubProvider) Capabilities() sfu.Capabilities {
	return sfu.Capabilities{
		ServerKick: true, DeleteRoom: true, ListRooms: true, ListMembers: true,
		KickLevel: sfu.EnforcementHard, DeleteLevel: sfu.EnforcementHard, ListLevel: sfu.EnforcementHard,
	}
}
func (s *stubProvider) GenerateToken(room, identity string) (string, error) {
	return "token", nil
}
func (s *stubProvider) ListRooms() ([]sfu.RoomSummary, error) { return nil, nil }
func (s *stubProvider) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	return nil, nil
}
func (s *stubProvider) MuteParticipant(room, identity, trackSid string, muted bool) error {
	return pkg.NewErrSFUNotSupported()
}
func (s *stubProvider) RemoveParticipant(room, identity string) error {
	return pkg.NewErrSFUNotSupported()
}
func (s *stubProvider) DeleteRoom(room string) error { return nil }
func (s *stubProvider) GetHost() string              { return "host" }

func TestFingerprintIncludesProviderAndCredentials(t *testing.T) {
	a := &config.Config{SFUProvider: "livekit", LiveKitHost: "h1", LiveKitKey: "k1", LiveKitSecret: "s1"}
	b := &config.Config{SFUProvider: "livekit", LiveKitHost: "h1", LiveKitKey: "k1", LiveKitSecret: "s2"}
	if fingerprint(a) == fingerprint(b) {
		t.Fatal("expected fingerprint to change when secret changes")
	}
}

func TestDynamicProvider_ProviderNamePrefersCachedProvider(t *testing.T) {
	cfg := &config.Config{SFUProvider: "mock"}
	p := NewDynamicProvider(func() (*config.Config, error) {
		return cfg, nil
	})
	p.mu.Lock()
	p.cachedProvider = &stubProvider{name: "mock"}
	p.cachedFingerprint = fingerprint(cfg)
	p.mu.Unlock()

	if got := p.ProviderName(); got != "mock" {
		t.Fatalf("ProviderName = %q, want mock from cached provider", got)
	}
}

func TestDynamicProvider_ProviderNameIgnoresStaleCache(t *testing.T) {
	p := NewDynamicProvider(func() (*config.Config, error) {
		return &config.Config{SFUProvider: "mock"}, nil
	})
	p.mu.Lock()
	p.cachedProvider = &stubProvider{name: "stale"}
	p.cachedFingerprint = "stale"
	p.mu.Unlock()

	if got := p.ProviderName(); got != "mock" {
		t.Fatalf("ProviderName = %q, want mock after config switch", got)
	}
}

func TestDynamicProvider_ProviderNameFallsBackToConfig(t *testing.T) {
	p := NewDynamicProvider(func() (*config.Config, error) {
		return &config.Config{SFUProvider: "mock"}, nil
	})
	if got := p.ProviderName(); got != "mock" {
		t.Fatalf("ProviderName = %q, want mock", got)
	}
}

func TestDynamicProvider_ProviderNameDefault(t *testing.T) {
	p := NewDynamicProvider(func() (*config.Config, error) {
		return nil, errors.New("boom")
	})
	if got := p.ProviderName(); got != "livekit" {
		t.Fatalf("ProviderName = %q, want livekit", got)
	}
}

func newDynamicProviderWithConfig(t *testing.T, provider string) *DynamicProvider {
	t.Helper()
	cfg := &config.Config{SFUProvider: provider}
	return NewDynamicProvider(func() (*config.Config, error) {
		return cfg, nil
	})
}

func TestDynamicProvider_SupportsStream(t *testing.T) {
	p := newDynamicProviderWithConfig(t, "livekit")
	if p.SupportsStream() {
		t.Fatal("livekit must not advertise stream support")
	}
}

type stubRoomResolver struct{ room string }

func (r *stubRoomResolver) RoomForStream(stream string) (string, bool) {
	if r == nil || r.room == "" {
		return "", false
	}
	return r.room, true
}

type resolverSetterStub struct {
	stubProvider
	got pkg.StreamRoomResolver
}

func (s *resolverSetterStub) SetStreamRoomResolver(r pkg.StreamRoomResolver) {
	s.got = r
}

func TestDynamicProvider_StreamRoomResolverForwardedAndStored(t *testing.T) {
	p := newDynamicProviderWithConfig(t, "mock")
	stub := &resolverSetterStub{}
	p.mu.Lock()
	p.cachedProvider = stub
	p.mu.Unlock()

	resolver := &stubRoomResolver{room: "dom-a:r1"}
	p.SetStreamRoomResolver(resolver)

	if stub.got != resolver {
		t.Fatal("resolver must be forwarded to the cached provider")
	}
	if p.streamRoomResolver != resolver {
		t.Fatal("resolver must be stored for re-injection after provider rebuild")
	}
}
