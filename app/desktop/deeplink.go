package main

import (
	"context"
	"regexp"
	"strings"
	"sync"

	"github.com/egoist/mygo"
)

// deeplinkOpenEvent carries normalized invite paths to the pages; the web side
// listens on "deeplink.open" (packages/desktop onDeepLink) and navigates.
var deeplinkOpenEvent = mygo.NewEvent[string]("deeplink.open")

// deeplinkScheme is the custom URL scheme registered by the packaged app
// (urlSchemes in mygo.config.ts) and at run time for unbundled builds.
const deeplinkScheme = "gospeak"

// invitePathPattern finds the invite landing path inside a deep link
// (gospeak://invite/d/<CODE>, whose "invite" is the URL host), an https invite
// URL or an already normalized path.
var invitePathPattern = regexp.MustCompile(`(?i)/invite/d/([A-HJ-NP-Z2-9]{36})(?:[/?#]|$)`)

// DeepLink handles gospeak:// URLs that launch or focus the app.
//
// A page consumes the pending path with Initial once it is ready to navigate
// (cold start), and every later URL is broadcast to the live pages. Both paths
// deliver the same normalized "/invite/d/<CODE>" path, so the web side never
// has to parse the URL scheme.
type DeepLink struct{}

var deepLinkState struct {
	sync.Mutex
	pending string
	ready   bool
}

// Initial returns the invite path the app was launched with (or ""), and marks
// the pages as ready to receive later links through the "deeplink.open" event.
func (DeepLink) Initial(ctx context.Context) string {
	deepLinkState.Lock()
	defer deepLinkState.Unlock()
	deepLinkState.ready = true
	p := deepLinkState.pending
	deepLinkState.pending = ""
	return p
}

// Open is the bound entry point for a deep link opened while the app runs.
func (DeepLink) Open(ctx context.Context, raw string) {
	deliverDeepLink(raw)
}

// deliverDeepLink normalizes a deep link to an invite path, brings the main
// window to the front and hands the path to the pages (or parks it until a page
// asks for it with Initial).
func deliverDeepLink(raw string) {
	target := invitePathFromLink(raw)
	if target == "" {
		return
	}
	showMainWindow()

	deepLinkState.Lock()
	if !deepLinkState.ready {
		deepLinkState.pending = target
		deepLinkState.Unlock()
		return
	}
	deepLinkState.Unlock()
	_ = deeplinkOpenEvent.Broadcast(target)
}

// invitePathFromLink extracts "/invite/d/<CODE>" from a gospeak:// URL, an
// https invite URL or an already normalized path. It returns "" for anything
// else.
func invitePathFromLink(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}
	if m := invitePathPattern.FindStringSubmatch(text); m != nil {
		return invitePath(strings.ToUpper(m[1]))
	}
	return ""
}

func invitePath(code string) string {
	return "/invite/d/" + code
}

// registerDeepLink wires OS deep links (launch URL and later opens) before the
// app runs. Packaging registers the scheme through mygo.config.ts; the runtime
// registration covers `go run` / `mygo dev` where no bundle is installed.
func registerDeepLink() {
	_ = mygo.App.RegisterURLScheme(deeplinkScheme)
	mygo.App.OnOpenURL(func(raw string) { deliverDeepLink(raw) })
}
