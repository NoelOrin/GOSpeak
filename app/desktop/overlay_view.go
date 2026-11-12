package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
)

const (
	hudWidth     = 200
	hudRowHeight = 22
	hudMaxRows   = 12
	hudPadding   = 4
	hudAvatar    = 16
	hudGap       = 6
	hudIconSize  = 12
)

var micIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 19v3"/><path d="M19 10v2a7 7 0 0 1-14 0v-2"/><rect x="9" y="2" width="6" height="13" rx="3"/></svg>`))

var micOffIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="2" x2="22" y1="2" y2="22"/><path d="M18.89 13.23A7.12 7.12 0 0 0 19 12v-2"/><path d="M5 10v2a7 7 0 0 0 12 5"/><path d="M15 9.34V5a3 3 0 0 0-5.68-1.33"/><path d="M9 9v3a3 3 0 0 0 5.12 2.12"/><path d="M12 19v3"/></svg>`))

// hudState is what the overlay HUD draws. The main window relays it through
// Overlay.PushState; the HUD itself never opens a socket or an SFU session.
type hudState struct {
	members  []OverlayMember
	speaking map[string]bool
	avatarOf func(identity string) *ui.Bitmap
}

// hudHeight returns the window height in content DIPs for n members: the
// padding plus one row each, capped at hudMaxRows (the last of which shows the
// overflow count).
func hudHeight(n int) int {
	if n > hudMaxRows {
		n = hudMaxRows
	}
	return hudPadding*2 + n*hudRowHeight
}

// orderMembers puts the members speaking first, keeping the rest in order, so
// that who talks is seen at a glance.
func orderMembers(members []OverlayMember, speaking map[string]bool) []OverlayMember {
	out := make([]OverlayMember, len(members))
	copy(out, members)
	sort.SliceStable(out, func(i, j int) bool {
		return speaking[out[i].Identity] && !speaking[out[j].Identity]
	})
	return out
}

// hudView draws the member list straight over the game: no card behind it, so
// every text carries a shadow in the opposite direction of the theme's text
// colour, which stays readable on a dark and on a light game alike.
func hudView(s *hudState) func(*ui.Context) {
	return func(c *ui.Context) {
		t := c.Theme()
		c.SetZoom(float32(hudScale()))
		c.Root().Background(ui.Transparent)
		if len(s.members) == 0 {
			return
		}
		shadow := ui.RGBA(0, 0, 0, 0.9)
		if !t.Dark {
			shadow = ui.RGBA(255, 255, 255, 0.9)
		}
		ordered := orderMembers(s.members, s.speaking)
		shown := ordered
		overflow := 0
		if len(shown) > hudMaxRows {
			overflow = len(shown) - (hudMaxRows - 1)
			shown = shown[:hudMaxRows-1]
		}
		ui.Column(c).Padding(hudPadding).Children(func() {
			for _, m := range shown {
				hudRow(c, s, m, shadow)
			}
			if overflow > 0 {
				ui.Text(c, fmt.Sprintf("+%d", overflow)).
					FontSize(12).
					TextColor(t.TextMuted).
					TextShadow(0, 1, 4, shadow)
			}
		})
	}
}

func hudRow(c *ui.Context, s *hudState, m OverlayMember, shadow ui.Color) {
	t := c.Theme()
	speaking := s.speaking[m.Identity]
	nameColor := t.TextMuted
	if speaking {
		nameColor = t.Text
	}
	ui.Row(c).Height(hudRowHeight).Gap(hudGap).AlignItems(ui.Center).Children(func() {
		hudAvatarView(c, s, m, speaking)
		icon := micIcon
		iconColor := t.TextMuted
		if m.IsMicMuted {
			icon = micOffIcon
		} else if speaking {
			iconColor = t.Text
		}
		ui.Icon(c, icon).Size(hudIconSize, hudIconSize).TextColor(iconColor)
		txt := ui.Text(c, displayNameOf(m)).
			FontSize(12).
			TextColor(nameColor).
			TextShadow(0, 1, 4, shadow).
			MaxWidth(hudWidth - hudPadding*2 - hudAvatar - hudGap*2 - hudIconSize).
			NoWrap()
		if speaking {
			txt.Bold()
		}
	})
}

func hudAvatarView(c *ui.Context, s *hudState, m OverlayMember, speaking bool) {
	t := c.Theme()
	box := ui.Box(c).Size(hudAvatar, hudAvatar)
	if speaking {
		box.Border(2, t.Success).Radius(hudAvatar / 2)
	}
	box.Children(func() {
		if s.avatarOf != nil {
			if bmp := s.avatarOf(m.Identity); bmp != nil {
				ui.Image(c, bmp).Size(hudAvatar, hudAvatar).Radius(hudAvatar / 2)
				return
			}
		}
		ui.Box(c).
			Size(hudAvatar, hudAvatar).
			Radius(hudAvatar/2).
			Background(t.Accent).
			Center().
			Children(func() {
				ui.Text(c, initialOf(displayNameOf(m))).FontSize(10).TextColor(t.AccentText)
			})
	})
}

func displayNameOf(m OverlayMember) string {
	if m.DisplayName != "" {
		return m.DisplayName
	}
	if m.Name != "" {
		return m.Name
	}
	return m.Identity
}

func initialOf(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "?"
	}
	for _, r := range name {
		return strings.ToUpper(string(r))
	}
	return "?"
}
