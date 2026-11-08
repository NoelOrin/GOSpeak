package main

import (
	"embed"
	"log"

	"github.com/egoist/mygo"
)

// Desktop-only image assets, embedded so `go run` and `mygo dev` show the
// right icons too: the packaged bundle gets its icon from resources/icon.png
// (mygo.config.ts), which only exists in a build.
//
//go:embed assets/*.png
var embeddedAssets embed.FS

// appIconPNG returns the application icon, or nil when it is missing.
func appIconPNG() []byte {
	data, err := embeddedAssets.ReadFile("assets/icon.png")
	if err != nil || len(data) == 0 {
		return nil
	}
	return data
}

// trayIconPNG returns the monochrome template icon the system tray shows.
func trayIconPNG() []byte {
	data, err := embeddedAssets.ReadFile("assets/tray.png")
	if err != nil || len(data) == 0 {
		return nil
	}
	return data
}

// applyAppIcon sets the Dock icon (macOS) and the window icon (Windows,
// Linux) from the embedded asset. A packaged app already carries the icon, so
// this only matters when running from source; failures are logged and
// non-fatal.
func applyAppIcon() {
	icon := appIconPNG()
	if icon == nil {
		log.Printf("[GOSpeak] app icon asset missing; keeping the platform default")
		return
	}
	if err := mygo.App.Dock.SetIcon(icon); err != nil {
		log.Printf("[GOSpeak] dock icon unavailable: %v", err)
	}
	if mainWindow != nil {
		if err := mainWindow.SetIcon(icon); err != nil {
			log.Printf("[GOSpeak] window icon unavailable: %v", err)
		}
	}
}
