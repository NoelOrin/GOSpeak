package main

import "testing"

func TestInvitePathFromLink(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"deep link host form", "gospeak://invite/d/B2JF5EDV", "/invite/d/B2JF5EDV"},
		{"deep link path form", "gospeak:///invite/d/B2JF5EDV", "/invite/d/B2JF5EDV"},
		{"deep link lowercase", "gospeak://invite/d/b2jf5edv", "/invite/d/B2JF5EDV"},
		{"https link", "https://gospeak.example.com/invite/d/B2JF5EDV", "/invite/d/B2JF5EDV"},
		{"http loopback link", "http://127.0.0.1:47000/invite/d/B2JF5EDV", "/invite/d/B2JF5EDV"},
		{"link with query", "https://gospeak.example.com/invite/d/B2JF5EDV?from=share", "/invite/d/B2JF5EDV"},
		{"plain path", "/invite/d/B2JF5EDV", "/invite/d/B2JF5EDV"},
		{"plain code", "B2JF5EDV", ""},
		{"wrong path", "https://gospeak.example.com/home", ""},
		{"bad alphabet", "gospeak://invite/d/B2JF5ED1", ""},
		{"short code", "gospeak://invite/d/ABC", ""},
		{"other scheme", "https://example.com/invite/d/B2JF5EDV", "/invite/d/B2JF5EDV"},
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

	deliverDeepLink("gospeak://invite/d/B2JF5EDV")

	if got := (DeepLink{}).Initial(nil); got != "/invite/d/B2JF5EDV" {
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
