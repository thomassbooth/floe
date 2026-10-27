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
	from := focusedPane(sess)
	win, err := sess.OpenWindow(w.title, dir, w.server.NewPaneID())
	if err != nil {
		return err
	}
	return w.start(sess, win, from)
}

// openAt adds a window at frame, on top of the others, and starts a pane
// sized to fit.
func (w *windowService) openAt(sess *domain.Session, frame domain.Rect) error {
	from := focusedPane(sess)
	win, err := sess.OpenAt(w.title, w.server.NewPaneID(), frame)
	if err != nil {
		return err
	}
	return w.start(sess, win, from)
}

// retitle applies title changes reported for panes, returning the sessions
// whose windows changed.
func (w *windowService) retitle(titles map[domain.PaneID]string) []domain.SessionID {
	var changed []domain.SessionID
	for pane, title := range titles {
		sess, ok := w.server.SessionForPane(pane)
		if !ok {
			continue
		}
		win, _ := sess.WindowForPane(pane)
		if sess.SetTitle(win.ID, title) {
			changed = append(changed, sess.ID())
		}
	}
	return changed
}

func focusedPane(sess *domain.Session) domain.PaneID {
	if win, ok := sess.Focused(); ok {
		return win.Pane
	}
	return 0
}

// start runs the pane for a newly opened window, closing the window again
// if the pane cannot start. The pane starts in the directory of the window
// that was focused before, so new windows open where you were working.
func (w *windowService) start(sess *domain.Session, win domain.Window, from domain.PaneID) error {
	var dir string
	if from != 0 {
		dir, _ = w.host.WorkingDir(from)
	}
	size := w.layout.contentSize(sess, win.ID)
	if err := w.host.Spawn(win.Pane, size, dir); err != nil {
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
