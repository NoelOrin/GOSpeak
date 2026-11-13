package main

import "github.com/egoist/mygo"

// autoScale picks the HUD zoom from a display's logical width: the same 12px
// text takes half the visual room on a 4K display at 100% scaling, so the HUD
// grows with the display instead of becoming unreadable.
func autoScale(displayWidth int) float64 {
	switch {
	case displayWidth <= 1920:
		return 1.0
	case displayWidth <= 2560:
		return 1.25
	case displayWidth <= 3440:
		return 1.35
	default:
		return 1.5
	}
}

// overlayDisplayWidth reports the logical width of the display the overlay
// sits on. It is a variable so tests can answer without a running app: mygo's
// Screen queries wait for the main loop, which a unit test has none of.
var overlayDisplayWidth = func() int {
	if win := overlayWindow(); win != nil {
		x, y := win.Position()
		if d := mygo.Screen.DisplayNearestPoint(mygo.Point{X: x, Y: y}); d.Bounds.Width > 0 {
			return d.Bounds.Width
		}
	} else if d := mygo.Screen.PrimaryDisplay(); d.Bounds.Width > 0 {
		return d.Bounds.Width
	}
	return 1920
}

// hudScale returns the zoom the HUD draws at: the user's override when set,
// the display's automatic one otherwise.
func hudScale() float64 {
	overlayCfg.mu.Lock()
	manual := overlayCfg.scale
	overlayCfg.mu.Unlock()
	if manual > 0 {
		return manual
	}
	return autoScale(overlayDisplayWidth())
}

// hudWindowSize returns the window size in window DIPs for n members: the
// content's size times the zoom, as SetContentSize takes the window's DIPs and
// the view lays out in the content's.
func hudWindowSize(n int) (int, int) {
	z := hudScale()
	return int(float64(hudWidth)*z + 0.5), int(float64(hudHeight(n))*z + 0.5)
}
