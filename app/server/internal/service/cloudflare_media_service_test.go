package service

import (
	"testing"

	"GOSpeak/internal/sfu/providers/cloudflare"
)

type fakeCloudflareMediaClient struct {
	addTracksCalled     bool
	renegotiateCalled   bool
	closeTracksCalled   bool
	deleteSessionCalled bool
}

func (f *fakeCloudflareMediaClient) AddTracks(sessionID string, req *cloudflare.TrackRequest) (*cloudflare.TracksResponse, error) {
	f.addTracksCalled = true
	return &cloudflare.TracksResponse{}, nil
}

func (f *fakeCloudflareMediaClient) Renegotiate(sessionID string, req *cloudflare.RenegotiateRequest) error {
	f.renegotiateCalled = true
	return nil
}

func (f *fakeCloudflareMediaClient) CloseTracks(sessionID string, req *cloudflare.CloseTrackRequest) (*cloudflare.CloseTrackResponse, error) {
	f.closeTracksCalled = true
	return &cloudflare.CloseTrackResponse{}, nil
}

func (f *fakeCloudflareMediaClient) DeleteSession(sessionID string) error {
	f.deleteSessionCalled = true
	return nil
}

func newTestCloudflareMediaService(client cloudflareMediaClient) *CloudflareMediaService {
	svc := NewCloudflareMediaService(nil)
	svc.clientFactory = func() (cloudflareMediaClient, error) {
		return client, nil
	}
	svc.sessionOwner = func(sessionID string) (string, bool) {
		return "uuid-owner", sessionID == "sess-1"
	}
	return svc
}

func TestCloudflareMediaService_Operations_NonOwnerForbidden(t *testing.T) {
	client := &fakeCloudflareMediaClient{}
	svc := newTestCloudflareMediaService(client)

	_, err := svc.AddTracks("sess-1", "uuid-other", &cloudflare.TrackRequest{})
	assertForbidden(t, err)

	err = svc.Renegotiate("sess-1", "uuid-other", &cloudflare.RenegotiateRequest{
		SessionDescription: cloudflare.SessionDescription{SDP: "sdp"},
	})
	assertForbidden(t, err)

	_, err = svc.CloseTracks("sess-1", "uuid-other", &cloudflare.CloseTrackRequest{})
	assertForbidden(t, err)

	err = svc.DeleteSession("sess-1", "uuid-other")
	assertForbidden(t, err)

	if client.addTracksCalled || client.renegotiateCalled || client.closeTracksCalled || client.deleteSessionCalled {
		t.Fatal("media client must not be called for non-owner")
	}
}

func TestCloudflareMediaService_Operations_OwnerSuccess(t *testing.T) {
	client := &fakeCloudflareMediaClient{}
	svc := newTestCloudflareMediaService(client)

	if _, err := svc.AddTracks("sess-1", "uuid-owner", &cloudflare.TrackRequest{}); err != nil {
		t.Fatalf("AddTracks failed: %v", err)
	}
	if err := svc.Renegotiate("sess-1", "uuid-owner", &cloudflare.RenegotiateRequest{
		SessionDescription: cloudflare.SessionDescription{SDP: "sdp"},
	}); err != nil {
		t.Fatalf("Renegotiate failed: %v", err)
	}
	if _, err := svc.CloseTracks("sess-1", "uuid-owner", &cloudflare.CloseTrackRequest{}); err != nil {
		t.Fatalf("CloseTracks failed: %v", err)
	}
	if err := svc.DeleteSession("sess-1", "uuid-owner"); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}
	if !client.addTracksCalled || !client.renegotiateCalled || !client.closeTracksCalled || !client.deleteSessionCalled {
		t.Fatal("expected all media operations to reach the client")
	}
}

func TestCloudflareMediaService_UnknownSessionForbidden(t *testing.T) {
	svc := newTestCloudflareMediaService(&fakeCloudflareMediaClient{})
	err := svc.DeleteSession("unknown", "uuid-owner")
	assertForbidden(t, err)
}

func TestCloudflareMediaService_MissingOwnerForbidden(t *testing.T) {
	svc := NewCloudflareMediaService(nil)
	svc.clientFactory = func() (cloudflareMediaClient, error) {
		return &fakeCloudflareMediaClient{}, nil
	}
	svc.sessionOwner = func(sessionID string) (string, bool) {
		return "", false
	}

	if _, err := svc.AddTracks("sess-1", "", &cloudflare.TrackRequest{}); err == nil {
		t.Fatal("expected missing user UUID to be rejected")
	} else {
		assertForbidden(t, err)
	}
	if _, err := svc.AddTracks("sess-1", "uuid-owner", &cloudflare.TrackRequest{}); err == nil {
		t.Fatal("expected unknown session to be rejected")
	} else {
		assertForbidden(t, err)
	}
}
