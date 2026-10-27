package domain

import (
	"fmt"
	"maps"
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
	// Activity is set when the window printed something while another one
	// had focus, and Bell when it rang the bell. Focusing it clears both.
	Activity, Bell bool
}

// Session is a named set of windows, where they are, and which one has
// focus. Windows move and resize freely and may overlap; the focused window
// is always on top. A session holds no processes: the service starts and
// stops panes to match.
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
	for _, id := range s.stack.IDs() {
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

// SetTitle changes a window's title. It reports whether it changed.
func (s *Session) SetTitle(id WindowID, title string) bool {
	w, ok := s.windows[id]
	if !ok || w.Title == title {
		return false
	}
	w.Title = title
	return true
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

// Focus gives a window focus and brings it to the front. It reports whether
// anything changed.
func (s *Session) Focus(id WindowID) bool {
	if _, ok := s.windows[id]; !ok {
		return false
	}
	raised := s.stack.Raise(id)
	w := s.windows[id]
	cleared := w.Activity || w.Bell
	w.Activity, w.Bell = false, false
	if cur, _ := s.focused(); cur == id {
		return raised || cleared
	}
	s.touch(id)
	return true
}

// MarkActivity notes that a pane printed something. Only windows without
// focus are marked. It reports whether the window changed.
func (s *Session) MarkActivity(pane PaneID) bool {
	w, ok := s.unfocusedWindowFor(pane)
	if !ok || w.Activity {
		return false
	}
	w.Activity = true
	return true
}

// MarkBell notes that a pane rang the bell. Only windows without focus are
// marked. It reports whether the window changed.
func (s *Session) MarkBell(pane PaneID) bool {
	w, ok := s.unfocusedWindowFor(pane)
	if !ok || w.Bell {
		return false
	}
	w.Bell = true
	return true
}

func (s *Session) unfocusedWindowFor(pane PaneID) (*Window, bool) {
	id, ok := s.panes[pane]
	if !ok {
		return nil, false
	}
	if cur, _ := s.focused(); cur == id {
		return nil, false
	}
	return s.windows[id], true
}

// FocusDirection moves focus to the neighbor of the focused window in
// direction dir, preferring recently focused windows on ties. It reports
// whether focus changed.
func (s *Session) FocusDirection(dir Direction) bool {
	cur, ok := s.focused()
	if !ok {
		return false
	}
	next, ok := Neighbor(s.placements(), cur, dir, s.recency)
	if !ok {
		return false
	}
	return s.Focus(next)
}

// Place moves and sizes a window, keeping it on screen. It reports whether
// the window changed.
func (s *Session) Place(id WindowID, frame Rect) bool {
	if _, ok := s.windows[id]; !ok {
		return false
	}
	delete(s.restore, id)
	return s.stack.SetFrame(id, fit(frame, s.area))
}

// Move shifts the focused window amount cells in direction dir, keeping it
// on screen. It reports whether the window moved.
func (s *Session) Move(dir Direction, amount int) bool {
	id, ok := s.focused()
	if !ok {
		return false
	}
	f, _ := s.frame(id)
	switch dir {
	case Left:
		f.X -= amount
	case Right:
		f.X += amount
	case Up:
		f.Y -= amount
	case Down:
		f.Y += amount
	}
	return s.Place(id, f)
}

// CycleFocus moves focus delta windows along in window order, wrapping
// around: 1 is the next window, -1 the previous. It reports whether focus
// changed.
func (s *Session) CycleFocus(delta int) bool {
	cur, ok := s.focused()
	if !ok {
		return false
	}
	ids := slices.Sorted(maps.Keys(s.windows))
	i := slices.Index(ids, cur)
	n := len(ids)
	return s.Focus(ids[((i+delta)%n+n)%n])
}

// ResizeEdge moves one edge of a window to pos, leaving the opposite edge
// where it is. Left and Up edges sit at pos; Right and Down edges end just
// before pos. The window stays at least MinWindow and on screen. It reports
// whether the window changed.
func (s *Session) ResizeEdge(id WindowID, edge Direction, pos int) bool {
	f, ok := s.frame(id)
	if !ok {
		return false
	}
	a := s.area
	switch edge {
	case Left:
		x := clamp(pos, a.X, f.Right()-MinWindow.W)
		f = Rect{X: x, Y: f.Y, W: f.Right() - x, H: f.H}
	case Right:
		f.W = clamp(pos, f.X+MinWindow.W, a.Right()) - f.X
	case Up:
		y := clamp(pos, a.Y, f.Bottom()-MinWindow.H)
		f = Rect{X: f.X, Y: y, W: f.W, H: f.Bottom() - y}
	case Down:
		f.H = clamp(pos, f.Y+MinWindow.H, a.Bottom()) - f.Y
	}
	return s.Place(id, f)
}

// Resize grows (Right, Down) or shrinks (Left, Up) the focused window by
// amount cells. A window against the screen edge grows the other way. It
// reports whether the window changed.
func (s *Session) Resize(dir Direction, amount int) bool {
	id, ok := s.focused()
	if !ok {
		return false
	}
	f, _ := s.frame(id)
	switch dir {
	case Right:
		if f.Right() >= s.area.Right() {
			return s.ResizeEdge(id, Left, f.X-amount)
		}
		return s.ResizeEdge(id, Right, f.Right()+amount)
	case Left:
		return s.ResizeEdge(id, Right, f.Right()-amount)
	case Down:
		if f.Bottom() >= s.area.Bottom() {
			return s.ResizeEdge(id, Up, f.Y-amount)
		}
		return s.ResizeEdge(id, Down, f.Bottom()+amount)
	case Up:
		return s.ResizeEdge(id, Down, f.Bottom()-amount)
	}
	return false
}

// ToggleMaximize makes a window fill the screen, or puts a maximized window
// back where it was before it was maximized or snapped. Moving or resizing a
// maximized window keeps it where it is. It reports whether the window
// changed.
func (s *Session) ToggleMaximize(id WindowID) bool {
	f, ok := s.frame(id)
	if !ok {
		return false
	}
	if prev, held := s.restore[id]; held && f == s.area {
		delete(s.restore, id)
		return s.stack.SetFrame(id, fit(prev, s.area))
	}
	return s.snapTo(id, s.area)
}

// Snap moves a window into a zone of the screen. The window remembers where
// it was, so dragging it away or restoring it gives that size back. It
// reports whether the window changed.
func (s *Session) Snap(id WindowID, zone Zone) bool {
	if zone == ZoneNone {
		return false
	}
	return s.snapTo(id, zone.Rect(s.area))
}

// Tidy snaps every window into a layout that fills the screen: Tile for
// the number of windows. Each window goes to the nearest free spot, so a
// rough arrangement keeps its shape. It reports whether anything changed.
func (s *Session) Tidy() bool {
	ids := s.stack.IDs()
	spots := Tile(len(ids), s.area)

	type pair struct {
		window, spot, dist int
	}
	var pairs []pair
	for wi, id := range ids {
		f, _ := s.frame(id)
		for si, spot := range spots {
			pairs = append(pairs, pair{wi, si, centerDistance(f, spot)})
		}
	}
	slices.SortStableFunc(pairs, func(a, b pair) int { return a.dist - b.dist })

	usedWindow := make([]bool, len(ids))
	usedSpot := make([]bool, len(spots))
	changed := false
	for _, p := range pairs {
		if usedWindow[p.window] || usedSpot[p.spot] {
			continue
		}
		usedWindow[p.window], usedSpot[p.spot] = true, true
		changed = s.snapTo(ids[p.window], spots[p.spot]) || changed
	}
	return changed
}

// snapTo moves a window to target, remembering its free frame unless it is
// already maximized or snapped.
func (s *Session) snapTo(id WindowID, target Rect) bool {
	f, ok := s.frame(id)
	if !ok || f == target {
		return false
	}
	if _, held := s.restore[id]; !held {
		s.restore[id] = f
	}
	return s.stack.SetFrame(id, target)
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
		Windows: make([]WindowView, 0, len(placements)),
	}
	for _, p := range placements {
		w := s.windows[p.Window]
		restore, held := s.restore[w.ID]
		if held {
			restore = fit(restore, s.area)
		}
		snap.Windows = append(snap.Windows, WindowView{
			ID:        w.ID,
			Pane:      w.Pane,
			Title:     w.Title,
			Activity:  w.Activity,
			Bell:      w.Bell,
			Frame:     p.Frame,
			Content:   p.Frame.Inset(FrameInset),
			Focused:   w.ID == focus,
			Maximized: held && p.Frame == s.area,
			Restore:   restore,
		})
	}
	return snap
}

// placements returns every window fitted to the area, back to front.
func (s *Session) placements() []Placement {
	ids := s.stack.IDs()
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
