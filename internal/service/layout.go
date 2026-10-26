package service

import "github.com/thomasbooth/multiplexer/internal/domain"

// layoutService manages where windows sit and makes sure every pane's
// screen size matches the space its window gives it.
type layoutService struct {
	host PaneHost
	// sizes is the size each pane was last told about.
	sizes map[domain.PaneID]domain.Size
}

func newLayoutService(host PaneHost) *layoutService {
	return &layoutService{host: host, sizes: make(map[domain.PaneID]domain.Size)}
}

// setArea sets the space a session's windows are laid out in. It reports
// whether that changed.
func (l *layoutService) setArea(sess *domain.Session, size domain.Size) bool {
	area := domain.Rect{W: size.W, H: size.H}
	if sess.Area() == area {
		return false
	}
	sess.SetArea(area)
	return true
}

// resize moves the focused window's nearest divider. It reports whether the
// layout changed.
func (l *layoutService) resize(sess *domain.Session, dir domain.Direction, amount int) bool {
	return sess.Resize(dir, amount)
}

// contentSize returns the space a window currently has for its pane.
func (l *layoutService) contentSize(sess *domain.Session, id domain.WindowID) domain.Size {
	for _, w := range sess.Snapshot().Windows {
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

// sync resizes every pane whose window changed size and returns the
// session's layout.
func (l *layoutService) sync(sess *domain.Session) domain.Snapshot {
	snap := sess.Snapshot()
	for _, w := range snap.Windows {
		size := w.Content.Size()
		if l.sizes[w.Pane] != size {
			l.sizes[w.Pane] = size
			l.host.Resize(w.Pane, size)
		}
	}
	return snap
}
