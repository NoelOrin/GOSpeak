package factory

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
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
	cfg := &config.Config{SFUProvider: "agora", AgoraAppID: "app"}
	p := NewDynamicProvider(func() (*config.Config, error) {
		return cfg, nil
	})
	p.mu.Lock()
	p.cachedProvider = &stubProvider{name: "agora"}
	p.cachedFingerprint = fingerprint(cfg)
	p.mu.Unlock()

	if got := p.ProviderName(); got != "agora" {
		t.Fatalf("ProviderName = %q, want agora from cached provider", got)
	}
}

func TestDynamicProvider_ProviderNameIgnoresStaleCache(t *testing.T) {
	p := NewDynamicProvider(func() (*config.Config, error) {
		return &config.Config{SFUProvider: "srs"}, nil
	})
	p.mu.Lock()
	p.cachedProvider = &stubProvider{name: "agora"}
	p.cachedFingerprint = "stale"
	p.mu.Unlock()

	if got := p.ProviderName(); got != "srs" {
		t.Fatalf("ProviderName = %q, want srs after config switch", got)
	}
}

func TestDynamicProvider_ProviderNameFallsBackToConfig(t *testing.T) {
	p := NewDynamicProvider(func() (*config.Config, error) {
		return &config.Config{SFUProvider: "cloudflare"}, nil
	})
	if got := p.ProviderName(); got != "cloudflare" {
		t.Fatalf("ProviderName = %q, want cloudflare", got)
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
	p2 := newDynamicProviderWithConfig(t, "srs")
	if !p2.SupportsStream() {
		t.Fatal("srs must advertise stream support")
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
	p := newDynamicProviderWithConfig(t, "srs")
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

type factoryStubRegistry struct {
	cleared []string
}

func (r *factoryStubRegistry) Rooms() []string              { return nil }
func (r *factoryStubRegistry) Streams(room string) []string { return nil }
func (r *factoryStubRegistry) ClearRoom(room string)        { r.cleared = append(r.cleared, room) }
func (r *factoryStubRegistry) StreamForIdentity(room, identity string) (string, bool) {
	return "", false
}
func (r *factoryStubRegistry) IdentityForStream(room, stream string) (string, bool) {
	return "", false
}

func TestDynamicProvider_RebuildReinjectsResolverAndRegistry(t *testing.T) {
	var (
		mu      sync.Mutex
		streams = []string{"gs-a1"}
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/streams/" && r.Method == http.MethodGet:
			mu.Lock()
			defer mu.Unlock()
			names := make([]map[string]string, 0, len(streams))
			for _, name := range streams {
				names = append(names, map[string]string{"app": "live", "name": name})
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "streams": names})
		case r.URL.Path == "/api/v1/clients/" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 0,
				"clients": []map[string]interface{}{{
					"id": "cid-1", "name": "gs-a1", "publish": true,
				}},
			})
		case r.URL.Path == "/api/v1/clients/cid-1" && r.Method == http.MethodDelete:
			mu.Lock()
			for i, st := range streams {
				if st == "gs-a1" {
					streams = append(streams[:i], streams[i+1:]...)
					break
				}
			}
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 0})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{SFUProvider: "srs", SRSHost: host, SRSApiPort: port}
	p := NewDynamicProvider(func() (*config.Config, error) { return cfg, nil })

	resolver := &stubRoomResolver{room: "dom-a:r1"}
	reg := &factoryStubRegistry{}
	p.SetStreamRoomResolver(resolver)
	p.SetRoomRegistry(reg)

	rooms, err := p.ListRooms()
	if err != nil {
		t.Fatalf("first ListRooms: %v", err)
	}
	if len(rooms) != 1 || rooms[0].Name != "dom-a:r1" {
		t.Fatalf("first ListRooms = %+v, want dom-a:r1", rooms)
	}

	// 修改 fingerprint 后再次调用：必须触发 current() 重建 provider，
	// 而不是沿用旧实例；新建的 provider 必须收到 resolver 重注入。
	cfg.SRSSecret = "rotated-secret"
	rooms, err = p.ListRooms()
	if err != nil {
		t.Fatalf("ListRooms after rebuild: %v", err)
	}
	if len(rooms) != 1 || rooms[0].Name != "dom-a:r1" {
		t.Fatalf("ListRooms after rebuild = %+v, want dom-a:r1 (resolver must be reinjected)", rooms)
	}

	// registry 重注入验证：重建后的 provider 调 DeleteRoom 必须清理 registry。
	if err := p.DeleteRoom("dom-a:r1"); err != nil {
		t.Fatalf("DeleteRoom after rebuild: %v", err)
	}
	if len(reg.cleared) != 1 || reg.cleared[0] != "dom-a:r1" {
		t.Fatalf("registry cleared = %v, want [dom-a:r1] (registry must be reinjected)", reg.cleared)
	}
}
