package domain

import (
	"slices"
	"testing"
)

var screen = Rect{W: 80, H: 24}

// newSession opens one window, then one more per split. Window n shows
// pane n.
func newSession(t *testing.T, splits ...Direction) *Session {
	t.Helper()
	s := NewSession(1, "test")
	s.SetArea(screen)
	for i, d := range append([]Direction{Right}, splits...) {
		if _, err := s.OpenWindow("sh", d, PaneID(i+1)); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func focusedID(t *testing.T, s *Session) WindowID {
	t.Helper()
	w, ok := s.Focused()
	if !ok {
		t.Fatal("expected a focused window")
	}
	return w.ID
}

func frames(s *Session) map[WindowID]Rect {
	out := map[WindowID]Rect{}
	for _, w := range s.Snapshot().Windows {
		out[w.ID] = w.Frame
	}
	return out
}

func order(s *Session) []WindowID {
	var out []WindowID
	for _, w := range s.Snapshot().Windows {
		out = append(out, w.ID)
	}
	return out
}

func TestSessionFirstWindowFillsTheArea(t *testing.T) {
	s := newSession(t)
	if got := frames(s)[1]; got != screen {
		t.Fatalf("frame = %+v", got)
	}
}

func TestSessionSplitHalvesTheFocusedWindow(t *testing.T) {
	tests := []struct {
		dir      Direction
		old, new Rect
	}{
		{Right, Rect{W: 40, H: 24}, Rect{X: 40, W: 40, H: 24}},
		{Left, Rect{X: 40, W: 40, H: 24}, Rect{W: 40, H: 24}},
		{Down, Rect{W: 80, H: 12}, Rect{Y: 12, W: 80, H: 12}},
		{Up, Rect{Y: 12, W: 80, H: 12}, Rect{W: 80, H: 12}},
	}
	for _, tt := range tests {
		t.Run(tt.dir.String(), func(t *testing.T) {
			s := newSession(t, tt.dir)
			got := frames(s)
			if got[1] != tt.old || got[2] != tt.new {
				t.Fatalf("got %v", got)
			}
			if focusedID(t, s) != 2 {
				t.Fatal("new window should have focus")
			}
		})
	}
}

func TestSessionSplitTooSmallOpensOffsetWindow(t *testing.T) {
	s := NewSession(1, "test")
	s.SetArea(Rect{W: 20, H: 10})
	s.OpenWindow("sh", Right, 1)
	s.OpenWindow("sh", Right, 2)
	got := frames(s)
	if got[1] != (Rect{W: 20, H: 10}) {
		t.Fatalf("too small to split, window 1 should be untouched: %+v", got[1])
	}
	if got[2].W != 20 || got[2].H != 10 {
		t.Fatalf("window 2 = %+v", got[2])
	}
}

func TestSessionCloseGivesSpaceToALinedUpNeighbor(t *testing.T) {
	// 1 | (2 / 3): closing 3 gives its space to 2, then closing 2 gives the
	// right half to 1.
	s := newSession(t, Right, Down)
	s.CloseWindow(3)
	if got := frames(s)[2]; got != (Rect{X: 40, W: 40, H: 24}) {
		t.Fatalf("window 2 = %+v", got)
	}
	s.CloseWindow(2)
	if got := frames(s)[1]; got != screen {
		t.Fatalf("window 1 = %+v", got)
	}
}

func TestSessionCloseGivesSpaceToAGroupThatCoversTheSide(t *testing.T) {
	// 1 | (2 / 3): closing 1 hands the left half to 2 and 3 together.
	s := newSession(t, Right, Down)
	s.CloseWindow(1)
	got := frames(s)
	if got[2] != (Rect{W: 80, H: 12}) || got[3] != (Rect{Y: 12, W: 80, H: 12}) {
		t.Fatalf("got %v", got)
	}
}

func TestSessionCloseIgnoresNeighborsThatStickOut(t *testing.T) {
	// (1 / 3) | 2, with 2 full height. Closing 1: window 2 touches its right
	// side but sticks out below it, so 3 underneath takes the space instead.
	s := newSession(t, Right)
	s.Focus(1)
	s.OpenWindow("sh", Down, 3)
	s.CloseWindow(1)
	got := frames(s)
	if got[3] != (Rect{W: 40, H: 24}) {
		t.Fatalf("window 3 should grow up, got %+v", got[3])
	}
	if got[2] != (Rect{X: 40, W: 40, H: 24}) {
		t.Fatalf("window 2 should be untouched, got %+v", got[2])
	}
}

func TestSessionClosePrefersRecentNeighbor(t *testing.T) {
	// 1 | 2 | 3 in a row: closing 2 can merge with either side.
	s := newSession(t)
	s.Place(1, Rect{W: 20, H: 24})
	s.OpenWindow("sh", Right, 2)
	s.OpenWindow("sh", Right, 3)
	s.Place(2, Rect{X: 20, W: 20, H: 24})
	s.Place(3, Rect{X: 40, W: 20, H: 24})
	s.Focus(1)
	s.Focus(3)
	s.CloseWindow(2)
	if got := frames(s)[3]; got != (Rect{X: 20, W: 40, H: 24}) {
		t.Fatalf("window 3 should take the space, got %+v", got)
	}
}

func TestSessionCloseLeavesUnalignedWindowsAlone(t *testing.T) {
	s := newSession(t, Right)
	s.Place(1, Rect{X: 2, Y: 2, W: 20, H: 10})
	s.CloseWindow(2)
	if got := frames(s)[1]; got != (Rect{X: 2, Y: 2, W: 20, H: 10}) {
		t.Fatalf("window 1 = %+v", got)
	}
}

func TestSessionOpenWindowRejectsDuplicatePane(t *testing.T) {
	s := newSession(t)
	if _, err := s.OpenWindow("sh", Right, 1); err == nil {
		t.Fatal("a pane may only be shown once")
	}
	if got, ok := s.WindowForPane(1); !ok || got.ID != 1 {
		t.Fatal("pane lookup failed")
	}
}

func TestSessionFocusBringsWindowToFront(t *testing.T) {
	s := newSession(t, Right, Down) // 1 | (2 / 3), 3 on top
	if !slices.Equal(order(s), []WindowID{1, 2, 3}) {
		t.Fatalf("order = %v", order(s))
	}
	if !s.Focus(1) || !slices.Equal(order(s), []WindowID{2, 3, 1}) {
		t.Fatalf("order = %v", order(s))
	}
	if s.Focus(1) {
		t.Fatal("focusing the top window again changes nothing")
	}
	if hit, ok := s.Snapshot().WindowAt(10, 10); !ok || hit.ID != 1 {
		t.Fatal("hit test should find window 1")
	}
}

func TestSessionCloseReturnsFocusToMostRecent(t *testing.T) {
	s := newSession(t, Right, Right) // windows 1, 2, 3; 3 focused
	s.Focus(1)
	s.Focus(3)

	if err := s.CloseWindow(3); err != nil {
		t.Fatal(err)
	}
	if got := focusedID(t, s); got != 1 {
		t.Fatalf("focus = %d, want 1", got)
	}
	if o := order(s); o[len(o)-1] != 1 {
		t.Fatalf("the newly focused window should be on top: %v", o)
	}

	s.CloseWindow(1)
	s.CloseWindow(2)
	if _, ok := s.Focused(); ok || s.Len() != 0 {
		t.Fatal("session should be empty")
	}
}

func TestSessionFocusDirectionPrefersRecentWindowOnTies(t *testing.T) {
	// 1 | (2 / 3)
	s := newSession(t, Right, Down)
	if !s.FocusDirection(Left) || focusedID(t, s) != 1 {
		t.Fatal("expected focus to move left to 1")
	}
	// 2 and 3 share the edge equally; 3 was focused more recently.
	if !s.FocusDirection(Right) || focusedID(t, s) != 3 {
		t.Fatalf("expected focus to return to 3, got %d", focusedID(t, s))
	}
	if !s.FocusDirection(Up) || focusedID(t, s) != 2 {
		t.Fatal("expected focus to move up to 2")
	}
	if s.FocusDirection(Up) {
		t.Fatal("nothing above 2, focus should not change")
	}
}

func TestSessionPlaceKeepsWindowOnScreen(t *testing.T) {
	s := newSession(t, Right)
	if !s.Place(2, Rect{X: 10, Y: 3, W: 30, H: 10}) {
		t.Fatal("expected a change")
	}
	if got := frames(s)[2]; got != (Rect{X: 10, Y: 3, W: 30, H: 10}) {
		t.Fatalf("got %+v", got)
	}
	if !s.Place(2, Rect{X: 70, Y: -4, W: 30, H: 10}) {
		t.Fatal("expected a change")
	}
	if got := frames(s)[2]; got != (Rect{X: 50, Y: 0, W: 30, H: 10}) {
		t.Fatalf("got %+v", got)
	}
	if s.Place(99, Rect{}) {
		t.Fatal("unknown window")
	}
}

func TestSessionResizeEdge(t *testing.T) {
	start := Rect{X: 20, Y: 5, W: 30, H: 10}
	tests := []struct {
		name string
		edge Direction
		pos  int
		want Rect
	}{
		{"right edge out", Right, 60, Rect{X: 20, Y: 5, W: 40, H: 10}},
		{"right edge stops at the screen", Right, 999, Rect{X: 20, Y: 5, W: 60, H: 10}},
		{"right edge stops at the minimum", Right, 0, Rect{X: 20, Y: 5, W: MinWindow.W, H: 10}},
		{"left edge out keeps the right edge", Left, 10, Rect{X: 10, Y: 5, W: 40, H: 10}},
		{"left edge in stops at the minimum", Left, 999, Rect{X: 50 - MinWindow.W, Y: 5, W: MinWindow.W, H: 10}},
		{"top edge up", Up, 0, Rect{X: 20, Y: 0, W: 30, H: 15}},
		{"bottom edge down stops at the screen", Down, 99, Rect{X: 20, Y: 5, W: 30, H: 19}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSession(t)
			s.Place(1, start)
			s.ResizeEdge(1, tt.edge, tt.pos)
			if got := frames(s)[1]; got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSessionKeyboardResize(t *testing.T) {
	s := newSession(t)
	s.Place(1, Rect{X: 10, Y: 5, W: 30, H: 10})

	s.Resize(Right, 4)
	s.Resize(Down, 2)
	if got := frames(s)[1]; got != (Rect{X: 10, Y: 5, W: 34, H: 12}) {
		t.Fatalf("grow: %+v", got)
	}
	s.Resize(Left, 4)
	s.Resize(Up, 2)
	if got := frames(s)[1]; got != (Rect{X: 10, Y: 5, W: 30, H: 10}) {
		t.Fatalf("shrink: %+v", got)
	}

	// Against the right and bottom edges, growing extends left and up.
	s.Place(1, Rect{X: 50, Y: 14, W: 30, H: 10})
	s.Resize(Right, 4)
	s.Resize(Down, 2)
	if got := frames(s)[1]; got != (Rect{X: 46, Y: 12, W: 34, H: 12}) {
		t.Fatalf("grow at the edge: %+v", got)
	}
}

func TestSessionToggleMaximize(t *testing.T) {
	s := newSession(t, Right)
	if !s.ToggleMaximize(2) {
		t.Fatal("expected a change")
	}
	snap := s.Snapshot()
	if w := snap.Windows[1]; w.Frame != screen || !w.Maximized {
		t.Fatalf("maximized window = %+v", w)
	}
	if !s.ToggleMaximize(2) {
		t.Fatal("expected a change")
	}
	if w := s.Snapshot().Windows[1]; w.Frame != (Rect{X: 40, W: 40, H: 24}) || w.Maximized {
		t.Fatalf("restored window = %+v", w)
	}

	// Moving a maximized window keeps it where it was moved to.
	s.ToggleMaximize(2)
	s.Place(2, Rect{X: 5, Y: 5, W: 20, H: 8})
	if s.Snapshot().Windows[1].Maximized {
		t.Fatal("a moved window is no longer maximized")
	}
}

func TestSessionScalesWindowsWithTheArea(t *testing.T) {
	s := newSession(t, Right)
	s.ToggleMaximize(1)
	s.SetArea(Rect{W: 160, H: 48})

	got := frames(s)
	if got[1] != (Rect{W: 160, H: 48}) {
		t.Fatalf("maximized window should fill the new area: %+v", got[1])
	}
	if got[2] != (Rect{X: 80, W: 80, H: 48}) {
		t.Fatalf("window 2 should keep its half: %+v", got[2])
	}
	s.ToggleMaximize(1)
	if got := frames(s)[1]; got != (Rect{W: 80, H: 48}) {
		t.Fatalf("restore frame should scale too: %+v", got)
	}
}

func TestSessionSnapshot(t *testing.T) {
	s := newSession(t, Right)
	snap := s.Snapshot()
	if snap.Focused != 2 || len(snap.Windows) != 2 || snap.Session != 1 || snap.Name != "test" {
		t.Fatalf("unexpected snapshot %+v", snap)
	}
	w := snap.Windows[1]
	if w.Content != (Rect{X: 41, Y: 1, W: 38, H: 22}) {
		t.Fatalf("content = %+v", w.Content)
	}
	if f, ok := snap.FocusedWindow(); !ok || f.ID != 2 {
		t.Fatal("focused window lookup failed")
	}
}

func TestKeyString(t *testing.T) {
	tests := map[string]Key{
		"ctrl+b":     {Code: 'b', Mod: ModCtrl},
		"|":          {Code: '|', Text: "|"},
		"shift+left": {Code: KeyLeft, Mod: ModShift},
		"esc":        {Code: KeyEscape},
	}
	for want, k := range tests {
		if got := k.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}
