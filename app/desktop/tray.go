package main

import (
	"context"
	"fmt"
	"log"
	"runtime"

	"github.com/egoist/mygo"
)

// TrayItem mirrors the menu model sent from the web frontend
// (packages/desktop createSystemTray). Keep field names in sync with the TS
// side so the generated client stays typed.
type TrayItem struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Separator bool   `json:"separator,omitempty"`
	Disabled  bool   `json:"disabled,omitempty"`
}

// trayMenuClickEvent carries the clicked item's ID to the pages; the web side
// listens on "tray.menuClick" (packages/desktop tray.onMenuClick).
var trayMenuClickEvent = mygo.NewEvent[string]("tray.menuClick")

// trayHandle is the system tray icon, created once in WhenReady with the app
// favicon embedded in the frontend dist.
var trayHandle *mygo.Tray

// ensureTray creates the system tray icon (macOS menu bar / Windows taskbar /
// Linux appindicator). Platforms without tray support are ignored: NewTray
// reports an error and the app keeps working without it.
func ensureTray() {
	if trayHandle != nil {
		return
	}
	icon := trayIconPNG()
	if len(icon) == 0 {
		log.Printf("[GOSpeak] tray icon asset missing")
		return
	}
	t, err := mygo.NewTray(mygo.TrayOptions{
		ID: "gospeak-tray",
		// A template image: macOS tints it for the menu bar and Windows
		// whitens it on a dark taskbar, so the monochrome mark follows the
		// system appearance instead of sitting there as a coloured square.
		Icon:           icon,
		IconIsTemplate: true,
		ToolTip:        "GOSpeak",
		Menu:           trayMenu(),
	})
	if err != nil {
		log.Printf("[GOSpeak] tray unavailable: %v", err)
		return
	}
	trayHandle = t
}

// Tray exposes the system tray to the embedded web frontend. The icon itself
// is created at startup (ensureTray); these methods adjust tooltip and menu.
type Tray struct{}

func (Tray) SetTooltip(ctx context.Context, tooltip string) {
	if trayHandle != nil {
		trayHandle.SetToolTip(tooltip)
	}
}

func (Tray) SetMenu(ctx context.Context, items []TrayItem) {
	if trayHandle == nil {
		return
	}
	mis := make([]*mygo.MenuItem, 0, len(items))
	for _, it := range items {
		if it.Separator {
			mis = append(mis, mygo.Separator())
			continue
		}
		id := it.ID
		mis = append(mis, &mygo.MenuItem{
			ID:       id,
			Label:    it.Label,
			Disabled: it.Disabled,
			Click: func(*mygo.MenuItem, *mygo.Window) {
				_ = trayMenuClickEvent.Broadcast(id)
			},
		})
	}
	trayHandle.SetMenu(mygo.NewMenu(mis))
}

func (Tray) Show(ctx context.Context) { ensureTray() }

func (Tray) Hide(ctx context.Context) {
	if trayHandle == nil {
		return
	}
	trayHandle.Destroy()
	trayHandle = nil
}

// unsupportedError 构造带平台的未实现错误。
func unsupportedError(feature string) error {
	return fmt.Errorf("%s is not implemented on %s", feature, runtime.GOOS)
}
