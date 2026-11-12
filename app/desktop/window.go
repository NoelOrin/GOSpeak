package main

import (
	"context"

	"github.com/egoist/mygo"
)

// Window exposes native window controls to the embedded web frontend.
//
// Every method takes a context.Context as its first parameter: MyGo cancels it
// when the calling page navigates away or its window closes, and
// mygo.CallerWindow returns the calling window from it — so the controls act on
// whichever window (main or overlay) issued the call.
//
// Command names (lowercased type + method) become `window.minimize` etc. on
// the TypeScript side; keep in sync with packages/desktop window.ts.
type Window struct{}

func (Window) Minimize(ctx context.Context) {
	if w := mygo.CallerWindow(ctx); w != nil {
		w.Minimize()
	}
}

func (Window) Maximize(ctx context.Context) {
	if w := mygo.CallerWindow(ctx); w != nil {
		w.Maximize()
	}
}

func (Window) Unmaximize(ctx context.Context) {
	if w := mygo.CallerWindow(ctx); w != nil {
		w.Unmaximize()
	}
}

func (Window) IsMaximized(ctx context.Context) bool {
	w := mygo.CallerWindow(ctx)
	return w != nil && w.IsMaximized()
}

func (Window) SetAlwaysOnTop(ctx context.Context, v bool) {
	if w := mygo.CallerWindow(ctx); w != nil {
		w.SetAlwaysOnTop(v)
	}
}

func (Window) SetTitle(ctx context.Context, title string) {
	if w := mygo.CallerWindow(ctx); w != nil {
		w.SetTitle(title)
	}
}

func (Window) Close(ctx context.Context) {
	if w := mygo.CallerWindow(ctx); w != nil {
		w.Close()
	}
}

// SetAppearance makes the native appearance follow the app's theme: the
// window controls of a hidden title bar (light glyphs on Windows for a dark
// page), and the native menus and dialogs. "system" follows the desktop again.
func (Window) SetAppearance(ctx context.Context, theme string) {
	w := mygo.CallerWindow(ctx)
	if w == nil {
		return
	}
	switch theme {
	case "light":
		w.SetWindowButtonsTheme(mygo.ThemeLight)
		mygo.Theme.SetSource(mygo.ThemeLight)
	case "dark":
		w.SetWindowButtonsTheme(mygo.ThemeDark)
		mygo.Theme.SetSource(mygo.ThemeDark)
	default:
		w.SetWindowButtonsTheme(mygo.ThemeSystem)
		mygo.Theme.SetSource(mygo.ThemeSystem)
	}
}
