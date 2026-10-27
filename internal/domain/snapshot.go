package domain

// Snapshot is a read-only copy of a session's state, ready for drawing.
type Snapshot struct {
	// Session and Name identify the session shown.
	Session SessionID
	Name    string
	Area    Rect
	// Windows are in draw order, back to front.
	Windows []WindowView
	// Focused is zero when the session has no windows.
	Focused WindowID
}

// WindowView is one window as it appears on screen.
type WindowView struct {
	ID    WindowID
	Pane  PaneID
	Title string
	// Activity and Bell mark a window that printed something, or rang the
	// bell, while another one had focus.
	Activity, Bell bool
	Frame          Rect
	Content        Rect
	Focused        bool
	// Maximized is set while the window fills the screen and remembers
	// where to go back to.
	Maximized bool
	// Restore is where a maximized or snapped window goes back to when
	// restored or dragged away. It is empty for a free window.
	Restore Rect
}

// WindowAt returns the front-most window whose frame contains the cell at
// x, y.
func (s Snapshot) WindowAt(x, y int) (WindowView, bool) {
	for i := len(s.Windows) - 1; i >= 0; i-- {
		if s.Windows[i].Frame.Contains(x, y) {
			return s.Windows[i], true
		}
	}
	return WindowView{}, false
}

// FocusedWindow returns the window with focus.
func (s Snapshot) FocusedWindow() (WindowView, bool) {
	for _, w := range s.Windows {
		if w.Focused {
			return w, true
		}
	}
	return WindowView{}, false
}
