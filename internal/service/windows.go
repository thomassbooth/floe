package service

import "github.com/thomasbooth/multiplexer/internal/domain"

// windowService opens and closes windows and keeps them paired with running
// panes: a window exists exactly as long as its pane's process does.
type windowService struct {
	session *domain.Session
	host    PaneHost
	layout  *layoutService
	title   string
}

func newWindowService(session *domain.Session, host PaneHost, layout *layoutService, title string) *windowService {
	return &windowService{session: session, host: host, layout: layout, title: title}
}

// open splits the focused window, or fills the screen if there is none, and
// starts a pane sized to fit.
func (w *windowService) open(dir domain.Direction) error {
	win, err := w.session.OpenWindow(w.title, dir)
	if err != nil {
		return err
	}
	size := w.layout.contentSize(win.ID)
	if err := w.host.Spawn(win.Pane, size); err != nil {
		_ = w.session.CloseWindow(win.ID)
		return err
	}
	w.layout.track(win.Pane, size)
	return nil
}

// closeFocused ends the focused window's pane. The window itself closes
// when the pane's exit is reported, the same path as a shell exiting.
func (w *windowService) closeFocused() {
	if win, ok := w.session.Focused(); ok {
		w.host.Kill(win.Pane)
	}
}

// paneExited closes the window showing a pane that has exited. It reports
// whether a window closed.
func (w *windowService) paneExited(id domain.PaneID) bool {
	w.layout.forget(id)
	win, ok := w.session.WindowForPane(id)
	if !ok {
		return false
	}
	_ = w.session.CloseWindow(win.ID)
	return true
}
