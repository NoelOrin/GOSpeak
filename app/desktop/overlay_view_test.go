package main

import (
	"os"
	"sync"
	"testing"

	"github.com/egoist/mygo/ui"
)

func testSnapshot(names ...string) OverlaySnapshot {
	members := make([]OverlayMember, 0, len(names))
	for _, n := range names {
		members = append(members, OverlayMember{Identity: n, Name: n})
	}
	return OverlaySnapshot{RoomName: "r", Members: members, Connected: true}
}

func newHUD(snap OverlaySnapshot, speaking []string) *hudState {
	s := &hudState{
		speaking: map[string]bool{},
		avatarOf: func(string) *ui.Bitmap { return nil },
		locked:   true,
	}
	sp := make(map[string]bool, len(speaking))
	for _, id := range speaking {
		sp[id] = true
	}
	s.setState(snap.Members, sp)
	return s
}

func TestHUDShowsMemberNames(t *testing.T) {
	s := newHUD(testSnapshot("阿伟", "小林", "Kate"), nil)
	tt := ui.NewTester(hudView(s), hudWidth, hudHeight(3))
	for _, want := range []string{"阿伟", "小林", "Kate"} {
		if !tt.HasText(want) {
			t.Errorf("HUD does not show %q; Texts() = %v", want, tt.Texts())
		}
	}
}

func TestHUDEmptyDrawsNothing(t *testing.T) {
	s := newHUD(testSnapshot(), nil)
	tt := ui.NewTester(hudView(s), hudWidth, hudHeight(0))
	if got := tt.Texts(); len(got) != 0 {
		t.Fatalf("Texts() = %v, want none for an empty room", got)
	}
}

func TestHUDSpeakingComesFirst(t *testing.T) {
	s := newHUD(testSnapshot("阿伟", "小林", "Kate"), []string{"Kate"})
	tt := ui.NewTester(hudView(s), hudWidth, hudHeight(3))
	texts := tt.Texts()
	kate, awei := -1, -1
	for i, txt := range texts {
		switch txt {
		case "Kate":
			kate = i
		case "阿伟":
			awei = i
		}
	}
	if kate < 0 || awei < 0 {
		t.Fatalf("Texts() = %v, want Kate and 阿伟", texts)
	}
	if kate > awei {
		t.Fatalf("Texts() = %v, want the speaking Kate before 阿伟", texts)
	}
}

func TestHUDCapsRowsWithOverflow(t *testing.T) {
	names := make([]string, 15)
	for i := range names {
		names[i] = string(rune('A' + i))
	}
	s := newHUD(testSnapshot(names...), nil)
	tt := ui.NewTester(hudView(s), hudWidth, hudHeight(15))
	if !tt.HasText("+4") {
		t.Errorf("HUD does not show +4 for 15 members; Texts() = %v", tt.Texts())
	}
	if tt.HasText("O") {
		t.Errorf("HUD shows the 15th member instead of +N; Texts() = %v", tt.Texts())
	}
}

func TestHUDHeight(t *testing.T) {
	cases := []struct{ n, want int }{
		{0, 8}, {1, 30}, {3, 74}, {12, 272}, {15, 272},
	}
	for _, c := range cases {
		if got := hudHeight(c.n); got != c.want {
			t.Errorf("hudHeight(%d) = %d, want %d", c.n, got, c.want)
		}
	}
}

func TestHUDThemeShadows(t *testing.T) {
	s := newHUD(testSnapshot("阿伟"), nil)
	tt := ui.NewTester(hudView(s), hudWidth, hudHeight(1))
	if !tt.HasText("阿伟") {
		t.Fatal("dark frame does not show the member")
	}
	tt.SetDark(false)
	if !tt.HasText("阿伟") {
		t.Fatal("light frame does not show the member")
	}
}

func TestHUDDragStateFollowsLock(t *testing.T) {
	s := newHUD(testSnapshot("阿伟"), nil)
	if !s.snapshot().locked {
		t.Fatal("HUD must start locked (no drag handle)")
	}
	s.setLocked(false)
	if s.snapshot().locked {
		t.Fatal("unlocking must expose the drag handle to the renderer")
	}
	// Both lock states must render without panicking (the root is a drag handle
	// only when unlocked).
	for _, locked := range []bool{true, false} {
		s.setLocked(locked)
		tt := ui.NewTester(hudView(s), hudWidth, hudHeight(1))
		if !tt.HasText("阿伟") {
			t.Fatalf("locked=%v: HUD does not render the member", locked)
		}
	}
}

func TestHUDSetStateIsConcurrentSafe(t *testing.T) {
	s := newHUD(testSnapshot(), nil)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			sp := map[string]bool{"阿伟": true}
			s.setState(testSnapshot("阿伟", "小林").Members, sp)
		}()
		go func() {
			defer wg.Done()
			_ = s.snapshot()
			_ = ui.NewTester(hudView(s), hudWidth, hudHeight(2)).Texts()
		}()
	}
	wg.Wait()
	if got := len(s.snapshot().members); got != 2 {
		t.Fatalf("members = %d, want 2", got)
	}
}

// TestMain keeps the HUD tests off mygo's Screen queries, which wait for the
// main loop that a unit test never runs.
func TestMain(m *testing.M) {
	overlayDisplayWidth = func() int { return 1920 }
	os.Exit(m.Run())
}
