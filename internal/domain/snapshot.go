package domain

// Snapshot is a read-only copy of a session's state, ready for drawing.
type Snapshot struct {
	// Session and Name identify the session shown.
	Session SessionID
	Name    string
	Area    Rect
	// Windows are in draw order, back to front. Minimized windows come
	// first and are not drawn.
	Windows []WindowView
	// Focused is zero when the session has no windows.
	Focused WindowID
}

// WindowView is one window as it appears on screen.
type WindowView struct {
	ID    WindowID
	Pane  PaneID
	Title string
	// Busy marks a window whose program keeps printing, and Bell one that
	// rang the bell while another had focus.
	Busy, Bell bool
	Frame      Rect
	Content    Rect
	Focused    bool
	// Maximized is set while the window fills the screen and remembers
	// where to go back to.
	Maximized bool
	// Minimized is set for a window that is hidden until it is focused.
	Minimized bool
	// Restore is where a maximized or snapped window goes back to when
	// restored or dragged away. It is empty for a free window.
	Restore Rect
}

// WindowAt returns the front-most window whose frame contains the cell at
// x, y.
func (s Snapshot) WindowAt(x, y int) (WindowView, bool) {
	for i := len(s.Windows) - 1; i >= 0; i-- {
		if !s.Windows[i].Minimized && s.Windows[i].Frame.Contains(x, y) {
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
