package domain

import "testing"

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

func TestSessionOpenWindowFocusesIt(t *testing.T) {
	s := newSession(t)
	w, err := s.OpenWindow("zsh", Right, 42)
	if err != nil {
		t.Fatal(err)
	}
	if w.ID != 2 || w.Pane != 42 || w.Title != "zsh" {
		t.Fatalf("unexpected window %+v", w)
	}
	if focusedID(t, s) != 2 {
		t.Fatal("new window should have focus")
	}
	if got, ok := s.WindowForPane(42); !ok || got.ID != 2 {
		t.Fatal("pane lookup failed")
	}
	if _, err := s.OpenWindow("zsh", Right, 42); err == nil {
		t.Fatal("a pane may only be shown once")
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
	if _, ok := s.WindowForPane(3); ok {
		t.Fatal("closed window's pane should be gone")
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
	if focusedID(t, s) != 3 {
		t.Fatal("setup: expected 3 focused")
	}

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

func TestSessionSnapshot(t *testing.T) {
	s := newSession(t, Right)
	snap := s.Snapshot()

	if snap.Focused != 2 || len(snap.Windows) != 2 || snap.Session != 1 || snap.Name != "test" {
		t.Fatalf("unexpected snapshot %+v", snap)
	}
	w := snap.Windows[1]
	if w.Frame != (Rect{X: 40, W: 40, H: 24}) {
		t.Fatalf("frame = %+v", w.Frame)
	}
	if w.Content != (Rect{X: 41, Y: 1, W: 38, H: 22}) {
		t.Fatalf("content = %+v", w.Content)
	}
	if hit, ok := snap.WindowAt(10, 10); !ok || hit.ID != 1 {
		t.Fatal("hit test failed")
	}
	if f, ok := snap.FocusedWindow(); !ok || f.ID != 2 {
		t.Fatal("focused window lookup failed")
	}
}

func TestSessionResizeTargetsFocusedWindow(t *testing.T) {
	s := newSession(t, Right)
	s.Focus(1)
	if !s.Resize(Right, 10) {
		t.Fatal("expected a change")
	}
	if got := s.Snapshot().Windows[0].Frame.W; got != 50 {
		t.Fatalf("width = %d, want 50", got)
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
