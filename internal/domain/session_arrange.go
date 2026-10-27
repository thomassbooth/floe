package domain

import "slices"

// This file holds the ways a session's windows are moved and sized: by
// hand, by snapping to a zone, and by tidying them all into a layout.

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
	ids := s.visible()
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
