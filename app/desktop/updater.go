package main

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/egoist/mygo"
)

// UpdateInfo mirrors packages/desktop createUpdater().
type UpdateInfo struct {
	// Available reports whether a newer release was found by the last Check.
	Available bool `json:"available"`
	// Version of the newer release ("" when up to date).
	Version string `json:"version,omitempty"`
	// Notes is the release notes in Markdown ("" when up to date).
	Notes string `json:"notes,omitempty"`
	// Current is the version of the running build.
	Current string `json:"current"`
	// Supported reports whether this build can self-update at all:
	// mygo.config.ts must configure `updates` (publicKey + feed) and the
	// build must not be a dev build. When false, the frontend offers
	// OpenReleasePage instead of Install.
	Supported bool `json:"supported"`
}

// updaterProgressEvent pushes install download progress (fraction 0..1) to
// the pages; packages/desktop createUpdater() listens on "updater.progress".
var updaterProgressEvent = mygo.NewEvent[float64]("updater.progress")

// releasePageURL is where users download releases when the running build
// cannot self-update (dev builds, package-manager installs).
const releasePageURL = "https://github.com/NoelOrin/GOSpeak/releases/latest"

// pendingUpdate holds the *mygo.Update returned by the last Check, so a
// later Install can download it. MyGo binds Updater{} by value, so the state
// lives here rather than on the struct.
var pendingUpdate struct {
	sync.Mutex
	up *mygo.Update
}

// Updater checks the update feed configured in mygo.config.ts (`updates`)
// for a newer desktop release, installs it in place (signed, verified with
// the embedded public key) and relaunches.
//
// The feed is the newest GitHub release tagged `desktop-v<version>` on
// NoelOrin/GOSpeak — the same tag the desktop release chain publishes
// (release.yml), so a release becomes installable as soon as its
// update-<target>.json manifests are attached.
type Updater struct{}

// init pins the version mygo compares update manifests against to the version
// this build reports. mygo.Updater.Check compares against App.Version(), which
// for a packaged build falls back to packageVersion — the version field of
// mygo.config.ts, injected by `mygo build` — while UpdateInfo.Current reports
// the ldflags-injected `version`. Those are bumped by different files
// (mygo.config.ts vs .github/versions/desktop.yaml), so a build whose two
// sources drifted would offer its own version as an update forever.
// scripts/check-version.mjs now keeps both in sync at release time; this keeps
// a mispackaged build honest at runtime too.
//
// TODO(update-feed): if mygo.config.ts ever grows an `updates` key, the
// release chain must keep packageVersion in lockstep — see the desktop rule in
// scripts/check-version.mjs.
func init() {
	if v := comparisonVersion(version); v != "" {
		mygo.App.SetVersion(v)
	}
}

// comparisonVersion is the version mygo should compare update manifests
// against: the build's own version when it is a real release, or "" to leave
// mygo's default (the packaged version) alone for dev builds.
func comparisonVersion(ldflagsVersion string) string {
	v := strings.TrimSpace(ldflagsVersion)
	if v == "" || v == "dev" {
		return ""
	}
	return v
}

// Check asks the update feed for a version newer than the running one.
//
// Returns Supported=false (with Available=false, no error) for builds that
// cannot self-update — dev builds, or releases built before `updates` was
// configured in mygo.config.ts — so the frontend can fall back to opening
// the release page.
func (Updater) Check(ctx context.Context) (*UpdateInfo, error) {
	info := &UpdateInfo{Current: version, Supported: mygo.Updater.Enabled()}
	// mygo.config.ts has no `updates` key, so Enabled is false for every build
	// today and mygo.Updater.Check would return ErrUpdatesDisabled anyway. Stop
	// here so the version-source caveat above is enforced at the one place that
	// compares versions: nothing is compared while the feature is off.
	if !info.Supported {
		return info, nil
	}

	update, err := mygo.Updater.Check(ctx)
	if err != nil {
		if errors.Is(err, mygo.ErrUpdatesDisabled) {
			return info, nil
		}
		return nil, err
	}
	if update == nil {
		return info, nil
	}

	pendingUpdate.Lock()
	pendingUpdate.up = update
	pendingUpdate.Unlock()

	info.Available = true
	info.Version = update.Version
	info.Notes = update.Notes
	return info, nil
}

// Install downloads the pending update found by Check, verifies its
// signature, replaces the running app with it and relaunches. Download
// progress is broadcast as "updater.progress" (fraction 0..1).
func (Updater) Install(ctx context.Context) error {
	pendingUpdate.Lock()
	update := pendingUpdate.up
	pendingUpdate.Unlock()
	if update == nil {
		return errors.New("no pending update: call Updater.Check first")
	}

	err := update.Install(ctx, func(downloaded, total int64) {
		if total > 0 {
			_ = updaterProgressEvent.Broadcast(min(1, float64(downloaded)/float64(total)))
		}
	})
	if err != nil {
		return err
	}

	// The new version takes over now; Relaunch exits and starts the
	// freshly installed binary.
	mygo.App.Relaunch()
	return nil
}

// CurrentVersion returns the version of the running build (ldflags-injected
// at packaging time, "dev" when run from source).
func (Updater) CurrentVersion(ctx context.Context) string {
	return version
}

// OpenReleasePage opens the GitHub releases page in the system browser —
// the fallback for builds that cannot self-update (Supported=false).
func (Updater) OpenReleasePage(ctx context.Context) error {
	return openInBrowser(releasePageURL)
}

// openInBrowser opens url with the platform's default browser.
func openInBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
