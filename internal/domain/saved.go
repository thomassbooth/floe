package domain

import "fmt"

// SavedSession is a session written down, so it can be brought back after
// a restart: its name, the area it was laid out in, and its windows. The
// programs that ran in it are not saved; restored windows start fresh
// shells.
type SavedSession struct {
	Name string
	Area Rect
	// Windows are back to front.
	Windows []SavedWindow
}

// SavedWindow is a window written down.
type SavedWindow struct {
	// ID is kept so windows come back with the numbers people know them by.
	ID    WindowID
	Title string
	Frame Rect
	// Restore is where a maximized or snapped window goes back to; empty
	// for a free window.
	Restore Rect
	Focused bool
	// Pane is the pane the window showed when it was saved, so the caller
	// can look up details such as its directory. It means nothing after a
	// restart.
	Pane PaneID
	// Dir is the directory to start the restored window's shell in.
	Dir string
}

// Save writes down every session, oldest first.
func (s *Server) Save() []SavedSession {
	sessions := s.Sessions()
	out := make([]SavedSession, len(sessions))
	for i, sess := range sessions {
		out[i] = sess.save()
	}
	return out
}

func (s *Session) save() SavedSession {
	focus, _ := s.focused()
	saved := SavedSession{Name: s.name, Area: s.area}
	for _, id := range s.stack.IDs() {
		w := s.windows[id]
		frame, _ := s.stack.Frame(id)
		saved.Windows = append(saved.Windows, SavedWindow{
			ID:      id,
			Title:   w.Title,
			Frame:   frame,
			Restore: s.restore[id],
			Focused: id == focus,
			Pane:    w.Pane,
		})
	}
	return saved
}

// RestoreSession brings back a saved session, with a new pane for each
// window. The windows keep their frames, stacking and focus; their panes
// still have to be started.
func (s *Server) RestoreSession(saved SavedSession) (*Session, error) {
	sess, err := s.CreateSession(saved.Name)
	if err != nil {
		return nil, fmt.Errorf("restore session: %w", err)
	}
	sess.area = saved.Area
	var focus WindowID
	for _, sw := range saved.Windows {
		if _, taken := sess.windows[sw.ID]; taken || sw.ID <= 0 {
			_ = s.RemoveSession(sess.ID())
			return nil, fmt.Errorf("restore session %q: bad window number %d", saved.Name, sw.ID)
		}
		sess.lastWindow = sw.ID - 1
		w := sess.add(sw.Title, s.NewPaneID(), sw.Frame)
		if !sw.Restore.Empty() {
			sess.restore[w.ID] = sw.Restore
		}
		if sw.Focused {
			focus = w.ID
		}
	}
	// New windows are numbered after the highest restored one.
	sess.lastWindow = 0
	for id := range sess.windows {
		sess.lastWindow = max(sess.lastWindow, id)
	}
	if focus != 0 {
		sess.Focus(focus)
	}
	return sess, nil
}
