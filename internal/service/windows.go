package service

import (
	"fmt"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

// windowService opens and closes windows and keeps them paired with running
// panes: a window exists exactly as long as its pane's process does.
type windowService struct {
	server *domain.Server
	host   PaneHost
	layout *layoutService
	title  string
}

func newWindowService(server *domain.Server, host PaneHost, layout *layoutService, title string) *windowService {
	return &windowService{server: server, host: host, layout: layout, title: title}
}

// open splits the session's focused window, or fills the screen if there is
// none, and starts a pane sized to fit.
func (w *windowService) open(sess *domain.Session, dir domain.Direction) error {
	win, err := sess.OpenWindow(w.title, dir, w.server.NewPaneID())
	if err != nil {
		return err
	}
	return w.start(sess, win)
}

// openAt adds a window at frame, on top of the others, and starts a pane
// sized to fit.
func (w *windowService) openAt(sess *domain.Session, frame domain.Rect) error {
	win, err := sess.OpenAt(w.title, w.server.NewPaneID(), frame)
	if err != nil {
		return err
	}
	return w.start(sess, win)
}

// start runs the pane for a newly opened window, closing the window again
// if the pane cannot start.
func (w *windowService) start(sess *domain.Session, win domain.Window) error {
	size := w.layout.contentSize(sess, win.ID)
	if err := w.host.Spawn(win.Pane, size); err != nil {
		_ = sess.CloseWindow(win.ID)
		return err
	}
	w.layout.track(win.Pane, size)
	return nil
}

// close ends a window's pane, or the focused window's when id is zero. The
// window itself closes when the pane's exit is reported, the same path as a
// shell exiting.
func (w *windowService) close(sess *domain.Session, id domain.WindowID) error {
	if id == 0 {
		if win, ok := sess.Focused(); ok {
			w.host.Kill(win.Pane)
		}
		return nil
	}
	for _, win := range sess.Snapshot().Windows {
		if win.ID == id {
			w.host.Kill(win.Pane)
			return nil
		}
	}
	return fmt.Errorf("close window %d: %w", id, domain.ErrUnknownWindow)
}

// paneExited closes the window showing a pane that has exited. It returns
// the window's session and whether a window closed.
func (w *windowService) paneExited(id domain.PaneID) (*domain.Session, bool) {
	w.layout.forget(id)
	sess, ok := w.server.SessionForPane(id)
	if !ok {
		return nil, false
	}
	win, _ := sess.WindowForPane(id)
	_ = sess.CloseWindow(win.ID)
	return sess, true
}
