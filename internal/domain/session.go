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
	// Busy is set while the window's program keeps printing, whether or
	// not the window has focus. Bell is set when it rang the bell while
	// another window had focus; focusing it clears that.
	Busy, Bell bool
}

// Session is a named set of windows, where they are, and which one has
// focus. Windows move and resize freely and may overlap; the focused window
// is always on top. A minimized window keeps its frame and its program but
// is not drawn, and focus, tidying and splitting pass it by. A session holds
// no processes: the service starts and stops panes to match.
type Session struct {
	id      SessionID
	name    string
	area    Rect
	windows map[WindowID]*Window
	panes   map[PaneID]WindowID
	stack   *Stack
	// restore holds, for maximized and snapped windows, the frame they had
	// while free, to go back to.
	restore map[WindowID]Rect
	// history lists windows from least to most recently focused. The last
	// entry has focus.
	history []WindowID
	// minimized lists minimized windows, the most recently minimized last.
	minimized []WindowID

	lastWindow WindowID
}

// NewSession returns an empty session. Sessions are normally created
// through Server.CreateSession.
func NewSession(id SessionID, name string) *Session {
	return &Session{
		id:      id,
		name:    name,
		windows: make(map[WindowID]*Window),
		panes:   make(map[PaneID]WindowID),
		stack:   NewStack(),
		restore: make(map[WindowID]Rect),
	}
}

// ID returns the session's ID.
func (s *Session) ID() SessionID { return s.id }

// Name returns the session's name.
func (s *Session) Name() string { return s.name }

// Area returns the screen area windows are placed in.
func (s *Session) Area() Rect { return s.area }

// SetArea sets the screen area windows are placed in. Windows scale with
// it, so a layout keeps its proportions when the terminal is resized.
func (s *Session) SetArea(r Rect) {
	old := s.area
	s.area = r
	if old.Empty() || r.Empty() || old == r {
		return
	}
	for _, id := range s.stack.IDs() {
		f, _ := s.stack.Frame(id)
		s.stack.SetFrame(id, scale(f, old, r))
	}
	for id, f := range s.restore {
		s.restore[id] = scale(f, old, r)
	}
}

// Len returns the number of windows.
func (s *Session) Len() int { return len(s.windows) }

// OpenWindow adds a window showing pane and focuses it. The first window
// fills the area. Later ones split the focused window in half, taking the
// half on its dir side, or open just below and right of it when it is too
// small to split.
func (s *Session) OpenWindow(title string, dir Direction, pane PaneID) (Window, error) {
	if _, taken := s.panes[pane]; taken {
		return Window{}, fmt.Errorf("open window: pane %d is already shown", pane)
	}

	frame := s.area
	if cur, ok := s.focused(); ok {
		f, _ := s.stack.Frame(cur)
		if keep, open, ok := split(f, dir); ok {
			s.stack.SetFrame(cur, keep)
			delete(s.restore, cur)
			frame = open
		} else {
			frame = Rect{X: f.X + 2, Y: f.Y + 1, W: f.W, H: f.H}
		}
	}
	return s.add(title, pane, frame), nil
}

// OpenAt adds a window showing pane at frame, kept on screen, and focuses
// it. A window already drawn in exactly that spot pushes the new one down
// and to the right, so none hides another.
func (s *Session) OpenAt(title string, pane PaneID, frame Rect) (Window, error) {
	if _, taken := s.panes[pane]; taken {
		return Window{}, fmt.Errorf("open window: pane %d is already shown", pane)
	}
	frame = fit(frame, s.area)
	for range s.stack.Len() {
		if !s.occupied(frame) {
			break
		}
		frame = fit(Rect{X: frame.X + 2, Y: frame.Y + 1, W: frame.W, H: frame.H}, s.area)
	}
	return s.add(title, pane, frame), nil
}

// add puts a new window showing pane at frame, on top and focused.
func (s *Session) add(title string, pane PaneID, frame Rect) Window {
	s.lastWindow++
	w := &Window{ID: s.lastWindow, Pane: pane, Title: title}
	s.windows[w.ID] = w
	s.panes[w.Pane] = w.ID
	s.stack.Add(w.ID, fit(frame, s.area))
	s.touch(w.ID)
	return *w
}

// occupied reports whether some window is drawn exactly at frame.
func (s *Session) occupied(frame Rect) bool {
	for _, p := range s.placements() {
		if p.Frame == frame {
			return true
		}
	}
	return false
}

// CloseWindow removes a window. A neighbor that lines up with it exactly
// takes over its space, undoing the split that made it. Focus returns to the
// most recently focused window that is still open.
func (s *Session) CloseWindow(id WindowID) error {
	w, ok := s.windows[id]
	if !ok {
		return fmt.Errorf("close window %d: %w", id, ErrUnknownWindow)
	}
	gone, _ := s.stack.Frame(id)
	s.stack.Remove(id)
	delete(s.windows, id)
	delete(s.panes, w.Pane)
	delete(s.restore, id)
	s.history = slices.DeleteFunc(s.history, func(h WindowID) bool { return h == id })
	if s.isMinimized(id) {
		s.minimized = slices.DeleteFunc(s.minimized, func(m WindowID) bool { return m == id })
		return nil // it gave up no space on screen
	}
	s.absorb(gone)
	if cur, ok := s.focused(); ok {
		s.stack.Raise(cur)
	}
	return nil
}

// absorb gives the space a closed window left to the windows on one of its
// sides, when they line up with that side exactly: one window the same
// height or width, or several that together cover the side without
// sticking out. Of the sides that qualify, the one with the most recently
// focused window wins.
func (s *Session) absorb(gone Rect) {
	var best []WindowID
	bestRank := -1
	for _, side := range []Direction{Left, Right, Up, Down} {
		group, ok := s.coveringSide(gone, side)
		if !ok {
			continue
		}
		rank := len(s.history)
		for _, id := range group {
			rank = min(rank, s.recency(id))
		}
		if bestRank < 0 || rank < bestRank {
			best, bestRank = group, rank
		}
	}
	for _, id := range best {
		f, _ := s.stack.Frame(id)
		s.stack.SetFrame(id, grow(f, gone))
		delete(s.restore, id)
	}
}

// coveringSide returns the windows touching side of gone whose spans tile
// that side exactly.
func (s *Session) coveringSide(gone Rect, side Direction) ([]WindowID, bool) {
	type span struct {
		id         WindowID
		start, end int
	}
	var spans []span
	for _, id := range s.visible() {
		f, _ := s.stack.Frame(id)
		var touches bool
		var sp span
		switch side {
		case Left:
			touches, sp = f.Right() == gone.X, span{id, f.Y, f.Bottom()}
		case Right:
			touches, sp = f.X == gone.Right(), span{id, f.Y, f.Bottom()}
		case Up:
			touches, sp = f.Bottom() == gone.Y, span{id, f.X, f.Right()}
		case Down:
			touches, sp = f.Y == gone.Bottom(), span{id, f.X, f.Right()}
		}
		if touches {
			spans = append(spans, sp)
		}
	}
	lo, hi := gone.Y, gone.Bottom()
	if side.Axis() == AxisY {
		lo, hi = gone.X, gone.Right()
	}
	slices.SortFunc(spans, func(a, b span) int { return a.start - b.start })
	var ids []WindowID
	at := lo
	for _, sp := range spans {
		if sp.end <= lo || sp.start >= hi {
			continue // touches the line but beside the closed window
		}
		if sp.start != at || sp.end > hi {
			return nil, false
		}
		ids = append(ids, sp.id)
		at = sp.end
	}
	return ids, at == hi && len(ids) > 0
}

// WindowForPane returns the window showing a pane.
func (s *Session) WindowForPane(p PaneID) (Window, bool) {
	id, ok := s.panes[p]
	if !ok {
		return Window{}, false
	}
	return *s.windows[id], true
}

// Snapshot returns the session's state for drawing.
func (s *Session) Snapshot() Snapshot {
	focus, _ := s.focused()
	placements := s.placements()
	snap := Snapshot{
		Session: s.id,
		Name:    s.name,
		Area:    s.area,
		Focused: focus,
		Windows: make([]WindowView, 0, len(s.windows)),
	}
	// Minimized windows come first, behind every window drawn.
	hidden := make([]Placement, 0, len(s.minimized))
	for _, id := range s.stack.IDs() {
		if s.isMinimized(id) {
			f, _ := s.frame(id)
			hidden = append(hidden, Placement{Window: id, Frame: f})
		}
	}
	for _, p := range slices.Concat(hidden, placements) {
		w := s.windows[p.Window]
		restore, held := s.restore[w.ID]
		if held {
			restore = fit(restore, s.area)
		}
		snap.Windows = append(snap.Windows, WindowView{
			ID:        w.ID,
			Pane:      w.Pane,
			Title:     w.Title,
			Busy:      w.Busy,
			Bell:      w.Bell,
			Frame:     p.Frame,
			Content:   p.Frame.Inset(FrameInset),
			Focused:   w.ID == focus,
			Maximized: held && p.Frame == s.area,
			Minimized: s.isMinimized(w.ID),
			Restore:   restore,
		})
	}
	return snap
}

// placements returns every window shown, fitted to the area, back to
// front.
func (s *Session) placements() []Placement {
	ids := s.visible()
	out := make([]Placement, len(ids))
	for i, id := range ids {
		f, _ := s.frame(id)
		out[i] = Placement{Window: id, Frame: f}
	}
	return out
}

// frame returns where a window is drawn: its stored frame fitted to the
// area, which only differs while the area is too small for it.
func (s *Session) frame(id WindowID) (Rect, bool) {
	f, ok := s.stack.Frame(id)
	if !ok {
		return Rect{}, false
	}
	return fit(f, s.area), true
}

// focused returns the most recently focused window still shown.
func (s *Session) focused() (WindowID, bool) {
	for i := len(s.history) - 1; i >= 0; i-- {
		if id := s.history[i]; !s.isMinimized(id) {
			return id, true
		}
	}
	return 0, false
}

// visible returns the windows that are not minimized, back to front.
func (s *Session) visible() []WindowID {
	return slices.DeleteFunc(s.stack.IDs(), s.isMinimized)
}

func (s *Session) isMinimized(id WindowID) bool {
	return slices.Contains(s.minimized, id)
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
