package service

import "github.com/thomasbooth/multiplexer/internal/domain"

// layoutService manages where windows sit and makes sure every pane's
// screen size matches the space its window gives it.
type layoutService struct {
	session *domain.Session
	host    PaneHost
	// sizes is the size each pane was last told about.
	sizes map[domain.PaneID]domain.Size
}

func newLayoutService(session *domain.Session, host PaneHost) *layoutService {
	return &layoutService{session: session, host: host, sizes: make(map[domain.PaneID]domain.Size)}
}

// setArea sets the screen space windows are laid out in. It reports that
// the layout changed.
func (l *layoutService) setArea(size domain.Size) bool {
	l.session.SetArea(domain.Rect{W: size.W, H: size.H})
	return true
}

// resize moves the focused window's nearest divider. It reports whether the
// layout changed.
func (l *layoutService) resize(dir domain.Direction, amount int) bool {
	return l.session.Resize(dir, amount)
}

// contentSize returns the space a window currently has for its pane.
func (l *layoutService) contentSize(id domain.WindowID) domain.Size {
	for _, w := range l.session.Snapshot().Windows {
		if w.ID == id {
			return w.Content.Size()
		}
	}
	return domain.Size{}
}

// track records the size a pane was started with.
func (l *layoutService) track(id domain.PaneID, size domain.Size) {
	l.sizes[id] = size
}

// forget drops a pane that has exited.
func (l *layoutService) forget(id domain.PaneID) {
	delete(l.sizes, id)
}

// sync resizes every pane whose window changed size and returns the layout.
func (l *layoutService) sync() domain.Snapshot {
	snap := l.session.Snapshot()
	for _, w := range snap.Windows {
		size := w.Content.Size()
		if l.sizes[w.Pane] != size {
			l.sizes[w.Pane] = size
			l.host.Resize(w.Pane, size)
		}
	}
	return snap
}
