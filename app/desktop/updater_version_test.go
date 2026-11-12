package main

import (
	"context"
	"testing"
)

func TestComparisonVersion(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"release version is used", "1.2.3", "1.2.3"},
		{"release version with prerelease", "1.2.3-beta.1", "1.2.3-beta.1"},
		{"surrounding whitespace", " 1.2.3 ", "1.2.3"},
		{"dev build leaves mygo default", "dev", ""},
		{"empty leaves mygo default", "", ""},
		{"blank leaves mygo default", "   ", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := comparisonVersion(tc.in); got != tc.want {
				t.Fatalf("comparisonVersion(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestCheckReportsUnsupportedWithoutTouchingTheFeed covers the guard that keeps
// version comparison out of the picture while mygo.config.ts has no `updates`
// key: the call must answer from local state alone (Supported=false,
// Available=false, no error) and never reach the feed.
func TestCheckReportsUnsupportedWithoutTouchingTheFeed(t *testing.T) {
	ctx := context.Background()

	info, err := (Updater{}).Check(ctx)
	if err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
	if info == nil {
		t.Fatal("Check() = nil, want UpdateInfo")
	}
	if info.Supported {
		t.Fatal("Supported = true; mygo.config.ts has no updates key")
	}
	if info.Available {
		t.Fatal("Available = true on a build that cannot self-update")
	}
	if info.Version != "" {
		t.Fatalf("Version = %q, want empty", info.Version)
	}
	if info.Current != version {
		t.Fatalf("Current = %q, want %q", info.Current, version)
	}

	pendingUpdate.Lock()
	pending := pendingUpdate.up
	pendingUpdate.Unlock()
	if pending != nil {
		t.Fatal("Check() recorded a pending update on a build that cannot self-update")
	}
}
