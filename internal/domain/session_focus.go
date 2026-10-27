package domain

import "slices"

// This file holds what a session knows about each window apart from where
// it is: focus, minimizing, titles and the busy and bell marks.

// SetTitle changes a window's title. It reports whether it changed.
func (s *Session) SetTitle(id WindowID, title string) bool {
	w, ok := s.windows[id]
	if !ok || w.Title == title {
		return false
	}
	w.Title = title
	return true
}

// Focused returns the window with focus.
func (s *Session) Focused() (Window, bool) {
	id, ok := s.focused()
	if !ok {
		return Window{}, false
	}
	return *s.windows[id], true
}

// Focus gives a window focus and brings it to the front, restoring it if it
// was minimized. It reports whether anything changed.
func (s *Session) Focus(id WindowID) bool {
	if _, ok := s.windows[id]; !ok {
		return false
	}
	restored := s.isMinimized(id)
	s.minimized = slices.DeleteFunc(s.minimized, func(m WindowID) bool { return m == id })
	raised := s.stack.Raise(id) || restored
	w := s.windows[id]
	cleared := w.Bell
	w.Bell = false
	if cur, _ := s.focused(); cur == id {
		return raised || cleared
	}
	s.touch(id)
	return true
}

// Minimize hides a window, leaving its program running. Focus goes to the
// most recently focused window still shown. It reports whether the window
// changed.
func (s *Session) Minimize(id WindowID) bool {
	if _, ok := s.windows[id]; !ok || s.isMinimized(id) {
		return false
	}
	s.minimized = append(s.minimized, id)
	if cur, ok := s.focused(); ok {
		s.stack.Raise(cur)
	}
	return true
}

// RestoreMinimized shows and focuses the most recently minimized window. It
// reports whether there was one.
func (s *Session) RestoreMinimized() bool {
	if len(s.minimized) == 0 {
		return false
	}
	return s.Focus(s.minimized[len(s.minimized)-1])
}

// SetBusy marks whether a pane's program is busy printing. It reports
// whether the window changed.
func (s *Session) SetBusy(pane PaneID, busy bool) bool {
	id, ok := s.panes[pane]
	if !ok || s.windows[id].Busy == busy {
		return false
	}
	s.windows[id].Busy = busy
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

// CycleFocus moves focus delta windows along in window order, wrapping
// around: 1 is the next window, -1 the previous. It reports whether focus
// changed.
func (s *Session) CycleFocus(delta int) bool {
	cur, ok := s.focused()
	if !ok {
		return false
	}
	ids := slices.Sorted(slices.Values(s.visible()))
	i := slices.Index(ids, cur)
	n := len(ids)
	return s.Focus(ids[((i+delta)%n+n)%n])
}
