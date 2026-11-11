package main

import "testing"

func TestInvitePathFromLink(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"deep link host form", "gospeak://invite/d/ABCDEFGHJKLMNPQRSTUVWXYZ23456789ABCD", "/invite/d/ABCDEFGHJKLMNPQRSTUVWXYZ23456789ABCD"},
		{"deep link path form", "gospeak:///invite/d/ABCDEFGHJKLMNPQRSTUVWXYZ23456789ABCD", "/invite/d/ABCDEFGHJKLMNPQRSTUVWXYZ23456789ABCD"},
		{"deep link lowercase", "gospeak://invite/d/abcdefghjklmnpqrstuvwxyz23456789abcd", "/invite/d/ABCDEFGHJKLMNPQRSTUVWXYZ23456789ABCD"},
		{"https link", "https://gospeak.example.com/invite/d/ABCDEFGHJKLMNPQRSTUVWXYZ23456789ABCD", "/invite/d/ABCDEFGHJKLMNPQRSTUVWXYZ23456789ABCD"},
		{"http loopback link", "http://127.0.0.1:47000/invite/d/ABCDEFGHJKLMNPQRSTUVWXYZ23456789ABCD", "/invite/d/ABCDEFGHJKLMNPQRSTUVWXYZ23456789ABCD"},
		{"link with query", "https://gospeak.example.com/invite/d/ABCDEFGHJKLMNPQRSTUVWXYZ23456789ABCD?from=share", "/invite/d/ABCDEFGHJKLMNPQRSTUVWXYZ23456789ABCD"},
		{"plain path", "/invite/d/ABCDEFGHJKLMNPQRSTUVWXYZ23456789ABCD", "/invite/d/ABCDEFGHJKLMNPQRSTUVWXYZ23456789ABCD"},
		{"plain code", "ABCDEFGHJKLMNPQRSTUVWXYZ23456789ABCD", ""},
		{"wrong path", "https://gospeak.example.com/home", ""},
		{"bad alphabet", "gospeak://invite/d/B2JF5ED1", ""},
		{"short code", "gospeak://invite/d/ABC", ""},
		{"other scheme", "https://example.com/invite/d/ABCDEFGHJKLMNPQRSTUVWXYZ23456789ABCD", "/invite/d/ABCDEFGHJKLMNPQRSTUVWXYZ23456789ABCD"},
		{"empty", "  ", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := invitePathFromLink(tc.in); got != tc.want {
				t.Fatalf("invitePathFromLink(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestDeliverDeepLinkParksUntilReady(t *testing.T) {
	deepLinkState.Lock()
	deepLinkState.pending = ""
	deepLinkState.ready = false
	deepLinkState.Unlock()

	deliverDeepLink("gospeak://invite/d/ABCDEFGHJKLMNPQRSTUVWXYZ23456789ABCD")

	if got := (DeepLink{}).Initial(nil); got != "/invite/d/ABCDEFGHJKLMNPQRSTUVWXYZ23456789ABCD" {
		t.Fatalf("Initial() = %q", got)
	}
	if got := (DeepLink{}).Initial(nil); got != "" {
		t.Fatalf("Initial() second call = %q, want empty", got)
	}
}

func TestDeliverDeepLinkIgnoresNonInviteURLs(t *testing.T) {
	deepLinkState.Lock()
	deepLinkState.pending = ""
	deepLinkState.ready = false
	deepLinkState.Unlock()

	deliverDeepLink("gospeak://settings")

	if got := (DeepLink{}).Initial(nil); got != "" {
		t.Fatalf("Initial() = %q, want empty", got)
	}
}
