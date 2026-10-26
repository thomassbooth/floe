package domain

import (
	"fmt"
	"slices"
)

// WindowID identifies a window. Zero means no window.
type WindowID int

// PaneID identifies the content shown inside a window, such as a shell.
type PaneID int

// Window is a frame on screen showing one pane.
type Window struct {
	ID    WindowID
	Pane  PaneID
	Title string
}

// Session is the set of windows the user is working with, how they are laid
// out, and which one has focus. It holds no processes: the service starts
// and stops panes to match.
type Session struct {
	area    Rect
	windows map[WindowID]*Window
	panes   map[PaneID]WindowID
	tiled   Engine
	// history lists windows from least to most recently focused. The last
	// entry has focus.
	history []WindowID

	lastWindow WindowID
	lastPane   PaneID
}

// NewSession returns an empty session using a tiling layout.
func NewSession() *Session {
	return &Session{
		windows: make(map[WindowID]*Window),
		panes:   make(map[PaneID]WindowID),
		tiled:   NewTree(),
	}
}

// SetArea sets the screen area windows are laid out in.
func (s *Session) SetArea(r Rect) { s.area = r }

// Len returns the number of windows.
func (s *Session) Len() int { return len(s.windows) }

// OpenWindow adds a window with a new pane. The first window fills the area;
// later ones split the focused window, appearing on its dir side. The new
// window gets focus.
func (s *Session) OpenWindow(title string, dir Direction) (Window, error) {
	s.lastWindow++
	s.lastPane++
	w := &Window{ID: s.lastWindow, Pane: s.lastPane, Title: title}

	target, _ := s.focused()
	if err := s.tiled.Add(w.ID, target, dir); err != nil {
		return Window{}, fmt.Errorf("open window: %w", err)
	}
	s.windows[w.ID] = w
	s.panes[w.Pane] = w.ID
	s.touch(w.ID)
	return *w, nil
}

// CloseWindow removes a window. Focus returns to the most recently focused
// window that is still open.
func (s *Session) CloseWindow(id WindowID) error {
	w, ok := s.windows[id]
	if !ok {
		return fmt.Errorf("close window %d: %w", id, ErrUnknownWindow)
	}
	if err := s.tiled.Remove(id); err != nil {
		return fmt.Errorf("close window: %w", err)
	}
	delete(s.windows, id)
	delete(s.panes, w.Pane)
	s.history = slices.DeleteFunc(s.history, func(h WindowID) bool { return h == id })
	return nil
}

// WindowForPane returns the window showing a pane.
func (s *Session) WindowForPane(p PaneID) (Window, bool) {
	id, ok := s.panes[p]
	if !ok {
		return Window{}, false
	}
	return *s.windows[id], true
}

// Focused returns the window with focus.
func (s *Session) Focused() (Window, bool) {
	id, ok := s.focused()
	if !ok {
		return Window{}, false
	}
	return *s.windows[id], true
}

// Focus gives a window focus. It reports whether focus changed.
func (s *Session) Focus(id WindowID) bool {
	if _, ok := s.windows[id]; !ok {
		return false
	}
	if cur, _ := s.focused(); cur == id {
		return false
	}
	s.touch(id)
	return true
}

// FocusDirection moves focus to the neighbor of the focused window in
// direction dir, preferring recently focused windows on ties. It reports
// whether focus changed.
func (s *Session) FocusDirection(dir Direction) bool {
	cur, ok := s.focused()
	if !ok {
		return false
	}
	next, ok := Neighbor(s.tiled.Arrange(s.area), cur, dir, s.recency)
	if !ok {
		return false
	}
	return s.Focus(next)
}

// Resize moves the focused window's nearest divider. It reports whether the
// layout changed.
func (s *Session) Resize(dir Direction, amount int) bool {
	cur, ok := s.focused()
	if !ok {
		return false
	}
	return s.tiled.Resize(cur, dir, amount, s.area)
}

// Snapshot returns the session's state for drawing.
func (s *Session) Snapshot() Snapshot {
	focus, _ := s.focused()
	placements := s.tiled.Arrange(s.area)
	snap := Snapshot{
		Area:    s.area,
		Focused: focus,
		Windows: make([]WindowView, 0, len(placements)),
	}
	for _, p := range placements {
		w := s.windows[p.Window]
		snap.Windows = append(snap.Windows, WindowView{
			ID:      w.ID,
			Pane:    w.Pane,
			Title:   w.Title,
			Frame:   p.Frame,
			Content: p.Frame.Inset(FrameInset),
			Focused: w.ID == focus,
		})
	}
	return snap
}

func (s *Session) focused() (WindowID, bool) {
	if len(s.history) == 0 {
		return 0, false
	}
	return s.history[len(s.history)-1], true
}

// touch marks id as the most recently focused window.
func (s *Session) touch(id WindowID) {
	s.history = slices.DeleteFunc(s.history, func(h WindowID) bool { return h == id })
	s.history = append(s.history, id)
}

// recency ranks windows for tie-breaking: 0 is the most recently focused.
func (s *Session) recency(id WindowID) int {
	if i := slices.Index(s.history, id); i >= 0 {
		return len(s.history) - 1 - i
	}
	return len(s.history)
}
